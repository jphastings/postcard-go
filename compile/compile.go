package compile

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/h2non/bimg"
	"github.com/jphastings/postcard-go"
	"github.com/jphastings/postcard-go/internal/types"
	"github.com/jphastings/postcard-go/validate"
	"gopkg.in/yaml.v3"
)

var nameRegex = regexp.MustCompile(`(.+)-(?:front|back|meta)+\.[a-z]+`)

// FromFiles accepts a path to one of the three needed files, attempts to find the others, and provides the conventional name and bytes for the file.
func FromFiles(part string) (string, []byte, error) {
	dir := filepath.Dir(part)
	parts := nameRegex.FindStringSubmatch(filepath.Base(part))
	if len(parts) != 2 {
		return "", nil, fmt.Errorf("given filename not of the form *-{front,back,meta}.ext")
	}
	prefix := parts[1]

	meta, err := openVagueFilename(dir, prefix, "meta", "yml", "yaml")
	if err != nil {
		return "", nil, fmt.Errorf("couldn't load metadata: %w", err)
	}
	front, err := openVagueFilename(dir, prefix, "front", "png", "jpg", "tif", "tiff")
	if err != nil {
		return "", nil, fmt.Errorf("couldn't load postcard front: %w", err)
	}
	back, err := openVagueFilename(dir, prefix, "back", "png", "jpg", "tif", "tiff")
	if err != nil {
		return "", nil, fmt.Errorf("couldn't load postcard back: %w", err)
	}

	pc, err := FromReaders(front, back, meta)
	if err != nil {
		return "", nil, err
	}

	buf := new(bytes.Buffer)
	if err := postcard.Write(pc, buf); err != nil {
		return "", nil, err
	}

	return fmt.Sprintf("%s.postcard", prefix), buf.Bytes(), nil
}

// FromReaders accepts reader objects for each of the components of a postcard file, and creates an in-memory Postcard object.
func FromReaders(frontReader, backReader, metaReader io.Reader) (*types.Postcard, error) {
	var meta types.Metadata
	if err := yaml.NewDecoder(metaReader).Decode(&meta); err != nil {
		return nil, err
	}

	frontImg, err := readerToImage(frontReader, meta.Front.Secrets)
	if err != nil {
		return nil, err
	}
	backImg, err := readerToImage(backReader, meta.Back.Secrets)
	if err != nil {
		return nil, err
	}

	pc := &types.Postcard{
		Front: frontImg,
		Back:  backImg,
		Meta:  meta,
	}

	if err := validate.Dimensions(pc); err != nil {
		return nil, err
	}

	return pc, nil
}

func openVagueFilename(dir, prefix, suffix string, extensions ...string) (io.Reader, error) {
	for _, ext := range extensions {
		r, err := os.Open(path.Join(dir, fmt.Sprintf("%s-%s.%s", prefix, suffix, ext)))
		if err == nil {
			return r, nil
		}
	}
	return nil, fmt.Errorf("no file '%s-%s.{%s}' in %s", prefix, suffix, strings.Join(extensions, ","), dir)
}

// readerToImage converts any input image type to a webp image, with any secret regions blurred out
func readerToImage(r io.Reader, secrets []types.Polygon) ([]byte, error) {
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(r)
	if err != nil {
		return nil, err
	}

	img := bimg.NewImage(buf.Bytes())
	if len(secrets) != 0 {
		log.Println("BEWARE! Automatic blurring of secret regions of postcards is not yet implemented!")
	}

	return img.Convert(bimg.WEBP)
}