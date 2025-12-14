//go:build ignore

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	_ "image/png"
	"io"
	"os"
)

func main() {
	sourceFile := "Icon.png"
	destFile := "Icon.ico"

	fmt.Printf("Converting %s -> %s...\n", sourceFile, destFile)

	f, err := os.Open(sourceFile)
	if err != nil {
		fmt.Printf("Error opening PNG: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		fmt.Printf("Error decoding PNG: %v\n", err)
		os.Exit(1)
	}

	buf := new(bytes.Buffer)

	// ICONDIR Header
	binary.Write(buf, binary.LittleEndian, uint16(0)) // Reserved
	binary.Write(buf, binary.LittleEndian, uint16(1)) // Type (1=ICO)
	binary.Write(buf, binary.LittleEndian, uint16(1)) // Count (1 app image)

	// ICONDIRENTRY
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	
	// ICO format uses 0 for 256px
	bw, bh := byte(w), byte(h)
	if w >= 256 { bw = 0 }
	if h >= 256 { bh = 0 }

	buf.WriteByte(bw)
	buf.WriteByte(bh)
	buf.WriteByte(0) // Color palette
	buf.WriteByte(0) // Reserved
	binary.Write(buf, binary.LittleEndian, uint16(0)) // Planes
	binary.Write(buf, binary.LittleEndian, uint32(32)) // BPP

	// PNG Data size
	stat, _ := f.Stat()
	pngLen := uint32(stat.Size())
	binary.Write(buf, binary.LittleEndian, pngLen)

	// Offset (6 header + 16 entry = 22)
	binary.Write(buf, binary.LittleEndian, uint32(22))

	// Write PNG Data
	f.Seek(0, 0)
	io.Copy(buf, f)

	err = os.WriteFile(destFile, buf.Bytes(), 0644)
	if err != nil {
		fmt.Printf("Error writing ICO: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Icon conversion successful.")
}
