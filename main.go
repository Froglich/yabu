package main

import (
	"archive/zip"
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type zipperFile struct {
	RealPath  string
	TruncPath string
}

type zipper struct {
	Files []zipperFile
}

func (zf *zipper) AddFile(f zipperFile) {
	zf.Files = append(zf.Files, f)
}

func (zf *zipper) Compress(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}

	zipWriter := zip.NewWriter(file)
	defer zipWriter.Close()

	tot := len(zf.Files)

	fmt.Printf("Writing '%s'\n", filename)

	for x := range zf.Files {
		f := zf.Files[x]
		reader, err := os.Open(f.RealPath)
		if err != nil {
			return err
		}
		defer reader.Close()

		info, err := reader.Stat()
		if err != nil {
			return err
		}

		head, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		head.Name = f.TruncPath
		head.Method = zip.Deflate

		writer, err := zipWriter.CreateHeader(head)
		if err != nil {
			return err
		}

		progress := (float32(x) / float32(tot)) * 100
		fmt.Printf("\t[%s] Deflating '%s'\n", leftPad(fmt.Sprintf("%.1f%%", progress), 6), f.TruncPath)
		_, err = io.Copy(writer, reader)

		if err != nil {
			return err
		}

		reader.Close()
	}

	fmt.Println("\t[100.0%]")

	return nil
}

func (zf *zipper) CompressToTempFile() (string, error) {
	filename := filepath.Join(os.TempDir(), randString()+".zip")

	return filename, zf.Compress(filename)
}

func newZipper() zipper {
	return zipper{Files: make([]zipperFile, 0)}
}

func (zf *zipper) AddFilesRecursively(p string, r string) error {
	i, err := os.Stat(p)
	if err != nil {
		return err
	}

	if !i.IsDir() {
		if r != "" {
			zf.AddFile(zipperFile{RealPath: p, TruncPath: r})
		} else {
			zf.AddFile(zipperFile{RealPath: p, TruncPath: i.Name()})
		}
		return nil
	}

	files, err := ioutil.ReadDir(p)
	if err != nil {
		return err
	}

	for _, f := range files {
		err = zf.AddFilesRecursively(filepath.Join(p, f.Name()), path.Join(r, f.Name()))
		if err != nil {
			return err
		}
	}

	return nil
}

func randInt(min int, max int) int {
	rand.Seed(time.Now().UTC().UnixNano())
	return min + rand.Intn(max-min)
}

func randString() string {
	var bytes [20]byte

	for i := 0; i < 20; i++ {
		bytes[i] = byte(randInt(65, 90))
	}

	return string(bytes[:])
}

func leftPad(input string, length int) string {
	b := strings.Builder{}

	chars := length - len(input)

	for x := chars; x > 0; x-- {
		b.WriteString(" ")
	}

	b.WriteString(input)
	return b.String()
}

func encryptFile(input string, output string, password string) error {
	//Very basic, based on the Godocs example
	if len(password) > 32 {
		return fmt.Errorf("Password is too long, max 32 characters")
	}
	key := []byte(leftPad(password, 32))

	in, err := os.Open(input)
	if err != nil {
		return err
	}
	defer in.Close()

	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	var iv [aes.BlockSize]byte
	s := cipher.NewOFB(block, iv[:])

	out, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer out.Close()

	writer := &cipher.StreamWriter{S: s, W: out}
	if _, err := io.Copy(writer, in); err != nil {
		panic(err)
	}

	return nil
}

func decryptFile(input string, output string, password string) error {
	//Very basic, based on the Godocs example
	if len(password) > 32 {
		return fmt.Errorf("Password is too long, max 32 characters")
	}
	key := []byte(leftPad(password, 32))

	in, err := os.Open(input)
	if err != nil {
		return err
	}
	defer in.Close()

	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	var iv [aes.BlockSize]byte
	s := cipher.NewOFB(block, iv[:])

	out, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer out.Close()

	reader := &cipher.StreamReader{S: s, R: in}
	if _, err := io.Copy(out, reader); err != nil {
		return err
	}

	return nil
}

func compressAndEncryptZip(z *zipper, output string, pass string) error {
	zf, err := z.CompressToTempFile()
	if err != nil {
		return err
	}
	defer os.Remove(zf)

	fmt.Println("Encrypting the archive...")

	return encryptFile(zf, output, pass)
}

func decryptAndInflateZip(f string, output string, pass string) error {
	fmt.Println("Decrypting the archive...")
	tf := filepath.Join(os.TempDir(), randString()+".zip")

	err := decryptFile(f, tf, pass)
	if err != nil {
		return err
	}
	defer os.Remove(tf)

	return unZip(tf, output)
}

func unZip(filename string, output string) error {
	zf, err := zip.OpenReader(filename)
	if err != nil {
		return err
	}
	defer zf.Close()

	fmt.Printf("Unzipping '%s' to '%s'\n", filename, output)

	tot := len(zf.File)

	for x := range zf.File {
		f := zf.File[x]
		outPath := filepath.Join(output, f.Name)

		if strings.Contains(outPath, "../") || strings.Contains(outPath, "..\\") {
			return fmt.Errorf("illegal filename: '%s'", outPath)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(outPath, os.ModePerm)
			continue
		}

		if err = os.MkdirAll(filepath.Dir(outPath), os.ModePerm); err != nil {
			return err
		}

		out, err := os.OpenFile(outPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		in, err := f.Open()
		if err != nil {
			return err
		}

		progress := (float32(x) / float32(tot)) * 100
		fmt.Printf("\t[%s] Inflating '%s'\n", leftPad(fmt.Sprintf("%.1f%%", progress), 6), f.Name)
		_, err = io.Copy(out, in)

		out.Close()
		in.Close()

		if err != nil {
			return err
		}
	}

	fmt.Println("\t[100.0%]")

	return nil
}

func createBackup(output string, input string, pass string) {
	fmt.Printf("Backing up '%s' to '%s'\n", input, output)
	z := newZipper()

	err := z.AddFilesRecursively(input, "")

	if err != nil {
		panic(err)
	}

	if pass != "" {
		err = compressAndEncryptZip(&z, output, pass)
	} else {
		err = z.Compress(output)
	}

	if err != nil {
		panic(err)
	}
}

func extractBackup(backup string, output string, pass string) {
	fmt.Printf("Restoring '%s' to '%s'\n", backup, output)
	var err error

	if pass != "" {
		err = decryptAndInflateZip(backup, output, pass)
	} else {
		err = unZip(backup, output)
	}

	if err != nil {
		panic(err)
	}
}

func main() {
	var output string
	var input string
	var backup string
	var pass string

	args := os.Args[1:]

	for x := 0; x < len(args); x++ {
		switch args[x] {
		case "-o":
			output = args[x+1]
			x++
		case "-c":
			input = args[x+1]
			x++
		case "-x":
			backup = args[x+1]
			x++
		case "-p":
			pass = args[x+1]
			x++
		}
	}

	if input != "" {
		createBackup(output, input, pass)
	} else if backup != "" {
		extractBackup(backup, output, pass)
	}

	fmt.Println("Done.")
}
