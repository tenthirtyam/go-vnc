// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: Ryan Johnson

package vnc

import (
	"encoding/binary"
	"io"
)

// HextileEncoding represents the Hextile encoding as defined in RFC 6143 Section 7.7.4.
// Hextile divides rectangles into 16x16 pixel tiles and applies different compression
// techniques to each tile based on its content.
type HextileEncoding struct {
	Tiles []HextileTile
}

// HextileTile represents a single 16x16 (or smaller) tile within a Hextile encoding.
type HextileTile struct {
	Width      uint16
	Height     uint16
	Background Color

	// Foreground color for this tile.
	Foreground Color

	// Colors contains decoded pixel data for raw tiles.
	Colors []Color

	// Subrectangles contains colored subrectangles for this tile.
	Subrectangles []HextileSubrectangle
}

// HextileSubrectangle represents a colored subrectangle within a Hextile tile.
type HextileSubrectangle struct {
	// Color is the color of this subrectangle.
	Color Color

	// X is the horizontal position within the tile (0-15).
	X uint8

	// Y is the vertical position within the tile (0-15).
	Y uint8

	// Width is the width of the subrectangle (1-16).
	Width uint8

	// Height is the height of the subrectangle (1-16).
	Height uint8
}

// Hextile encoding constants as defined in RFC 6143.
const (
	HextileRaw                 = 1
	HextileBackgroundSpecified = 2
	HextileForegroundSpecified = 4
	HextileAnySubrects         = 8
	HextileSubrectsColoured    = 16

	HextileTileSize    = 16
	MaxSubrectsPerTile = 255
)

// Type returns the encoding type identifier for Hextile encoding.
func (*HextileEncoding) Type() int32 {
	return 5
}

// Read decodes Hextile encoding data from the server.
//   - error: EncodingError if the data cannot be read or is invalid
//
// Example usage:
//
//	// This method is typically called by the VNC client's message processing loop
//	enc := &HextileEncoding{}
//	decodedEnc, err := enc.Read(clientConn, rectangle, dataReader)
//	if err != nil {
//		log.Printf("Failed to decode Hextile encoding: %v", err)
//		return
//	}
//
//	// Process the Hextile encoding
//	hextileEnc := decodedEnc.(*HextileEncoding)
//
//	// Render each tile to the framebuffer
//	tileIndex := 0
//	for tileY := uint16(0); tileY < rectangle.Height; tileY += 16 {
//		for tileX := uint16(0); tileX < rectangle.Width; tileX += 16 {
//			tile := hextileEnc.Tiles[tileIndex]
//			renderTile(rectangle.X + tileX, rectangle.Y + tileY, tile)
//			tileIndex++
//		}
//	}
func (*HextileEncoding) Read(c *ClientConn, rect *Rectangle, r io.Reader) (Encoding, error) {
	validator := newInputValidator()

	if c.FrameBufferWidth > 0 && c.FrameBufferHeight > 0 {
		if err := validator.ValidateRectangle(rect.X, rect.Y, rect.Width, rect.Height,
			c.FrameBufferWidth, c.FrameBufferHeight); err != nil {
			return nil, encodingError("HextileEncoding.Read", "invalid rectangle dimensions", err)
		}
	}

	tilesX := (rect.Width + HextileTileSize - 1) / HextileTileSize
	tilesY := (rect.Height + HextileTileSize - 1) / HextileTileSize
	totalTiles := int(tilesX * tilesY)

	const maxTiles = 100000
	if totalTiles > maxTiles {
		return nil, encodingError("HextileEncoding.Read", "too many tiles for rectangle", nil)
	}

	tiles := make([]HextileTile, totalTiles)
	tileIndex := 0

	var background, foreground Color

	for tileY := uint16(0); tileY < tilesY; tileY++ {
		for tileX := uint16(0); tileX < tilesX; tileX++ {
			tileWidth := uint16(HextileTileSize)
			tileHeight := uint16(HextileTileSize)

			if tileX*HextileTileSize+HextileTileSize > rect.Width {
				tileWidth = rect.Width - tileX*HextileTileSize
			}
			if tileY*HextileTileSize+HextileTileSize > rect.Height {
				tileHeight = rect.Height - tileY*HextileTileSize
			}

			tile := &tiles[tileIndex]
			tile.Width = tileWidth
			tile.Height = tileHeight

			var subencoding uint8
			if err := binary.Read(r, binary.BigEndian, &subencoding); err != nil {
				return nil, encodingError("HextileEncoding.Read", "failed to read tile subencoding", err)
			}

			if subencoding&HextileRaw != 0 {
				pixelCount := int(tileWidth * tileHeight)
				tile.Colors = make([]Color, pixelCount)

				for i := 0; i < pixelCount; i++ {
					color, err := readPixelColor(r, c.PixelFormat, c.ColorMap)
					if err != nil {
						return nil, encodingError("HextileEncoding.Read", "failed to read raw tile pixel", err)
					}
					tile.Colors[i] = color
				}
			} else {
				if subencoding&HextileBackgroundSpecified != 0 {
					var err error
					background, err = readPixelColor(r, c.PixelFormat, c.ColorMap)
					if err != nil {
						return nil, encodingError("HextileEncoding.Read", "failed to read background color", err)
					}
				}
				tile.Background = background

				if subencoding&HextileForegroundSpecified != 0 {
					var err error
					foreground, err = readPixelColor(r, c.PixelFormat, c.ColorMap)
					if err != nil {
						return nil, encodingError("HextileEncoding.Read", "failed to read foreground color", err)
					}
				}
				tile.Foreground = foreground

				if subencoding&HextileAnySubrects != 0 {
					var numSubrects uint8
					if err := binary.Read(r, binary.BigEndian, &numSubrects); err != nil {
						return nil, encodingError("HextileEncoding.Read", "failed to read subrectangle count", err)
					}

					if numSubrects > MaxSubrectsPerTile {
						return nil, encodingError("HextileEncoding.Read", "too many subrectangles in tile", nil)
					}

					tile.Subrectangles = make([]HextileSubrectangle, numSubrects)

					for i := uint8(0); i < numSubrects; i++ {
						subrect := &tile.Subrectangles[i]

						if subencoding&HextileSubrectsColoured != 0 {
							var err error
							subrect.Color, err = readPixelColor(r, c.PixelFormat, c.ColorMap)
							if err != nil {
								return nil, encodingError("HextileEncoding.Read", "failed to read subrectangle color", err)
							}
						} else {
							subrect.Color = foreground
						}
						var xyData, whData uint8
						if err := binary.Read(r, binary.BigEndian, &xyData); err != nil {
							return nil, encodingError("HextileEncoding.Read", "failed to read subrectangle position", err)
						}
						if err := binary.Read(r, binary.BigEndian, &whData); err != nil {
							return nil, encodingError("HextileEncoding.Read", "failed to read subrectangle dimensions", err)
						}

						subrect.X = (xyData >> 4) & 0x0F
						subrect.Y = xyData & 0x0F
						subrect.Width = ((whData >> 4) & 0x0F) + 1
						subrect.Height = (whData & 0x0F) + 1

						tileWidthU8 := uint8(tileWidth)   // #nosec G115 - tileWidth is <= 16
						tileHeightU8 := uint8(tileHeight) // #nosec G115 - tileHeight is <= 16
						if subrect.X >= tileWidthU8 || subrect.Y >= tileHeightU8 {
							return nil, encodingError("HextileEncoding.Read", "subrectangle position outside tile bounds", nil)
						}
						if uint16(subrect.X)+uint16(subrect.Width) > tileWidth ||
							uint16(subrect.Y)+uint16(subrect.Height) > tileHeight {
							return nil, encodingError("HextileEncoding.Read", "subrectangle extends outside tile bounds", nil)
						}
					}
				}
			}

			tileIndex++
		}
	}

	return &HextileEncoding{Tiles: tiles}, nil
}
