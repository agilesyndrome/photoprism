package thumb

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"net/url"
	"path"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"

	"github.com/photoprism/photoprism/pkg/clean"
	"github.com/photoprism/photoprism/pkg/fs"
)

// Suffix returns the thumb cache file suffix.
func Suffix(width, height int, opts ...ResampleOption) (result string) {
	method, _, format := ResampleOptions(opts...)

	result = fmt.Sprintf("%dx%d_%s.%s", width, height, ResampleMethods[method], format)

	return result
}

// FileName returns the file name of the thumbnail for the matching size.
// If storage is provided, it will be used to determine the appropriate path format.
func FileName(hash, thumbPath string, width, height int, opts ...ResampleOption) (fileName string, err error) {
	if InvalidSize(width) {
		return "", fmt.Errorf("thumb: width exceeds limit (%d)", width)
	}

	if InvalidSize(height) {
		return "", fmt.Errorf("thumb: height exceeds limit (%d)", height)
	}

	if len(hash) < 4 {
		return "", fmt.Errorf("thumb: file hash is empty or too short (%s)", clean.Log(hash))
	}

	if len(thumbPath) == 0 {
		return "", errors.New("thumb: folder is empty")
	}

	suffix := Suffix(width, height, opts...)
	
	// For S3 storage, we don't need to create directories
	if u, err := url.Parse(thumbPath); err == nil && u.Scheme != "" {
		// For S3, use a path-like structure without creating directories
		return fmt.Sprintf("%s/%s/%s/%s_%s", 
			strings.TrimSuffix(thumbPath, "/"), 
			hash[0:1], hash[1:2], hash[2:3],
			hash, suffix), nil
	}

	// For filesystem storage, maintain backward compatibility
	p := path.Join(thumbPath, hash[0:1], hash[1:2], hash[2:3])

	if err = fs.MkdirAll(p); err != nil {
		return "", err
	}

	fileName = path.Join(p, fmt.Sprintf("%s_%s", hash, suffix))

	return fileName, nil
}

// ResolvedName returns the file name of the thumbnail for the matching size with all symlinks resolved.
func ResolvedName(hash, thumbPath string, width, height int, opts ...ResampleOption) (fileName string, err error) {
	if fileName, err = FileName(hash, thumbPath, width, height, opts...); err != nil {
		return fileName, err
	} else {
		return fs.Resolve(fileName)
	}
}

// FromCache returns the filename if a thumbnail image with the matching size is in the cache.
// It checks both the local filesystem and any configured storage backends.
func FromCache(imageFilename, hash, thumbPath string, width, height int, opts ...ResampleOption) (fileName string, err error) {
	if len(hash) < 4 {
		return "", fmt.Errorf("thumb: invalid file hash %s", clean.Log(hash))
	}

	if len(imageFilename) < 4 {
		return "", fmt.Errorf("thumb: invalid file name %s", clean.Log(imageFilename))
	}

	// Generate the thumbnail file name
	fileName, err = FileName(hash, thumbPath, width, height, opts...)
	if err != nil {
		log.Debugf("thumb: %s in %s (filename)", err, clean.Log(filepath.Base(imageFilename)))
		return "", err
	}

	// Check if we have a storage backend for the thumbnails
	if storage := thumbStorage(thumbPath); storage != nil {
		// Check if the file exists in the storage backend
		exists, err := storage.Exists(fileName)
		if err != nil {
			log.Debugf("thumb: failed to check file existence in storage: %v", err)
			return "", ErrNotCached
		}
		if exists {
			// For S3 storage, we need to return a URL that can be used to access the file
			if u, err := url.Parse(thumbPath); err == nil && u.Scheme == "s3" {
				// Return the full path including the scheme and bucket
				return fileName, nil
			}
			return fileName, nil
		}
	} else {
		// Fall back to filesystem check
		if resolved, err := fs.Resolve(fileName); err == nil {
			if fs.FileExistsNotEmpty(resolved) {
				return resolved, nil
			}
		}
	}

	return "", ErrNotCached
}

// FromFile generates a new thumbnail with the requested size, if it does not already exist, and returns its filename.
// If storage is provided, it will be used for checking cache and saving the thumbnail.
func FromFile(imageName, hash, thumbPath string, width, height, orientation int, opts ...ResampleOption) (fileName string, err error) {
	// Check if the thumbnail is already in the cache
	if fileName, err = FromCache(imageName, hash, thumbPath, width, height, opts...); err == nil {
		return fileName, nil
	} else if !errors.Is(err, ErrNotCached) {
		return "", err
	}

	// Generate the thumbnail file name
	fileName, err = FileName(hash, thumbPath, width, height, opts...)
	if err != nil {
		log.Debugf("thumb: %s in %s (filename)", err, clean.Log(filepath.Base(imageName)))
		return "", err
	}

	// Get the storage backend for thumbnails
	storage := thumbStorage(thumbPath)

	// Use libvips to generate thumbnails if available
	if Library == LibVips {
		// Pass the storage to Vips function
		fileName, _, err = Vips(imageName, nil, hash, thumbPath, width, height, opts...)
		if err != nil {
			return "", err
		}
		return fileName, nil
	}

	// Fall back to standard Go image processing
	img, err := Open(imageName, orientation)
	if err != nil {
		log.Debugf("thumb: %s in %s", err, clean.Log(filepath.Base(imageName)))
		return "", err
	}

	// Create the thumbnail
	if storage != nil {
		// Create a buffer to hold the encoded image
		var buf bytes.Buffer
		
		// Determine the image format and quality settings
		var (
			quality   imaging.EncodeOption
			imgFormat string
		)

		if fs.FileType(fileName) == fs.ImagePng {
			quality = imaging.PNGCompressionLevel(png.DefaultCompression)
			imgFormat = "png"
		} else {
			quality = JpegQuality(width, height).EncodeOption()
			imgFormat = "jpeg"
		}

		// Resample the image to the requested size
		result := Resample(img, width, height, opts...)

		// Encode the image to the buffer
		switch imgFormat {
		case "png":
			err = imaging.Encode(&buf, result, imaging.PNG, quality.(imaging.PNGCompressionLevel))
		case "jpeg":
			err = imaging.Encode(&buf, result, imaging.JPEG, quality.(int))
		default:
			err = fmt.Errorf("unsupported image format: %s", imgFormat)
		}

		if err != nil {
			log.Debugf("thumb: failed to encode %s: %v", clean.Log(filepath.Base(fileName)), err)
			return "", err
		}

		// Save the thumbnail to the storage backend
		if err = storage.Write(fileName, buf.Bytes(), fs.ModeFile); err != nil {
			log.Debugf("thumb: failed to save %s: %v", clean.Log(filepath.Base(fileName)), err)
			return "", err
		}

		return fileName, nil
	}

	// Fall back to filesystem-based thumbnail creation
	if _, err = Create(img, fileName, width, height, opts...); err != nil {
		return "", err
	}

	return fileName, nil
}

// Create creates an image thumbnail and saves it to the specified file.
// If storage is provided, it will be used for saving the thumbnail.
func Create(img image.Image, fileName string, width, height int, opts ...ResampleOption) (result image.Image, err error) {
	if InvalidSize(width) {
		return img, fmt.Errorf("thumb: width has an invalid value (%d)", width)
	}

	if InvalidSize(height) {
		return img, fmt.Errorf("thumb: height has an invalid value (%d)", height)
	}

	// Resample the image to the requested size
	result = Resample(img, width, height, opts...)

	// Determine the image format and quality settings
	var (
		quality   imaging.EncodeOption
		imgFormat string
	)

	if fs.FileType(fileName) == fs.ImagePng {
		quality = imaging.PNGCompressionLevel(png.DefaultCompression)
		imgFormat = "png"
	} else {
		quality = JpegQuality(width, height).EncodeOption()
		imgFormat = "jpeg"
	}

	// Create a buffer to hold the encoded image
	var buf bytes.Buffer
	var imgData []byte

	// Encode the image to the buffer
	switch imgFormat {
	case "png":
		err = imaging.Encode(&buf, result, imaging.PNG, quality.(imaging.PNGCompressionLevel))
	case "jpeg":
		err = imaging.Encode(&buf, result, imaging.JPEG, quality.(int))
	default:
		err = fmt.Errorf("unsupported image format: %s", imgFormat)
	}

	if err != nil {
		log.Debugf("thumb: failed to encode %s: %v", clean.Log(filepath.Base(fileName)), err)
		return result, err
	}

	imgData = buf.Bytes()

	// Check if we have a storage backend
	if storage := thumbStorage(filepath.Dir(fileName)); storage != nil {
		// Use storage backend to save the thumbnail
		err = storage.Write(fileName, imgData, fs.ModeFile)
	} else {
		// Fall back to direct filesystem access for backward compatibility
		// Ensure the directory exists
		if err = os.MkdirAll(filepath.Dir(fileName), fs.ModeDir); err != nil {
			return result, fmt.Errorf("failed to create directory: %v", err)
		}
		err = os.WriteFile(fileName, imgData, fs.ModeFile)
	}

	if err != nil {
		log.Debugf("thumb: failed to save %s: %v", clean.Log(filepath.Base(fileName)), err)
		return result, err
	}

	return result, nil
}
