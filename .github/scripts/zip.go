// Command zip packages a single file into a .zip archive under a chosen
// in-archive name. Used by the release workflow so Windows archives don't
// depend on a system `zip` binary being present on the build runner.
package main

import (
	"archive/zip"
	"io"
	"os"
)

func main() {
	if len(os.Args) != 4 {
		panic("usage: zip <src-file> <dst-zip> <name-in-zip>")
	}
	src, dst, name := os.Args[1], os.Args[2], os.Args[3]

	in, err := os.Open(src)
	if err != nil {
		panic(err)
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		panic(err)
	}

	out, err := os.Create(dst)
	if err != nil {
		panic(err)
	}
	defer out.Close()

	w := zip.NewWriter(out)
	hdr, err := zip.FileInfoHeader(info)
	if err != nil {
		panic(err)
	}
	hdr.Name = name
	hdr.Method = zip.Deflate

	fw, err := w.CreateHeader(hdr)
	if err != nil {
		panic(err)
	}
	if _, err := io.Copy(fw, in); err != nil {
		panic(err)
	}
	if err := w.Close(); err != nil {
		panic(err)
	}
}
