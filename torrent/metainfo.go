package torrent

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/Despire/tinytorrent/bencoding"
)

type InfoSingleFile struct {
	// The filename.
	Name string
	// Length of the file in bytes.
	Length int64
	// Optional
	// 32-character hex string corresponding to the MD5 sum of the file.
	Md5sum *string
}

type (
	FileInfo struct {
		// Length of the file in bytes.
		Length int64
		// Path to the file.
		Path string

		// Optional.
		// 32-character hex string corresponding to the MD5 sum of the file.
		Md5Sum *string
	}

	InfoMultiFile struct {
		// Name of the directory in which to store all the files.
		Name string
		// Describing each file in the downloaded directory.
		Files []FileInfo
	}
)

type Info struct {
	*InfoSingleFile
	*InfoMultiFile

	Metadata struct {
		// Hash is the SHA1 Hash of the value of the info key in the torrent file.
		// Use this when communicating with the tracker.
		Hash [20]byte
	}

	// Number of bytes in each piece.
	PieceLength int64
	// String consisting of the concatenation of all 20-byte SHA1 hash values,
	// one per piece (byte string, i.e. not urlencoded).
	// Is hexencoded for better readability.
	Pieces string

	// Optional
	// If it is set to "1", the client MUST publish its presence to get other peers
	//  ONLY via the trackers explicitly described in the metainfo file. If this field
	// is set to "0" or is not present, the client may obtain peer from other means,
	// e.g. PEX peer exchange, dht. Here, "private" may be read as "no external peer source".
	Private *int64
}

type MetaInfoFile struct {
	Info

	// Announce URL of the tracker.
	Announce string

	// Optional
	// Support for Web Seeds.
	// BEP19: https://www.bittorrent.org/beps/bep_0019.html
	UrlList []string
	// This is an extention to the official specification, offering backwards-compatibility.
	AnnounceList []string
	// The creation time of the torrent, in standard UNIX epoch format (seconds since 1-Jan-1970 00:00:00 UTC)
	CreationDate *time.Time
	// Free-form textual comments of the author.
	Comment *string
	// Name and version of the program used to create the .torrent.
	CreatedBy *string
	// The string encoding format used to generate the pieces part of the info dictionary in the .torrent metafile.
	Encoding *string
}

func From(bencoded io.Reader) (*MetaInfoFile, error) {
	v, err := bencoding.Decode(bencoded)
	if err != nil {
		return nil, err
	}

	if v.Type() != bencoding.DictionaryType {
		return nil, errors.New("passed in bencoded value is not of expected format, expected bencoded dictionary")
	}

	d := v.(*bencoding.Dictionary)

	info := MetaInfoFile{}

	for k, v := range d.Dict {
		if err := apply(k, v, &info); err != nil {
			return nil, err
		}
	}

	if err := validate(&info); err != nil {
		return nil, fmt.Errorf("failed to validate torrent file: %w", err)
	}

	return &info, nil
}

func apply(key string, value bencoding.Value, info *MetaInfoFile) error {
	switch key {
	case "info":
		l, ok := value.(*bencoding.Dictionary)
		if !ok {
			return fmt.Errorf("expected 'Info' to be of type Dictionary but was %T", value)
		}

		info.Metadata.Hash = sha1.Sum([]byte(l.Literal()))

		_, isMultiFile := l.Dict["files"]
		for k, v := range l.Dict {
			if err := infoCommon(k, v, &info.Info, isMultiFile); err != nil {
				return fmt.Errorf("failed to parse 'info' dictionary: %w", err)
			}
		}

		return nil
	case "announce":
		l, ok := value.(*bencoding.ByteString)
		if !ok {
			return fmt.Errorf("expected announce to be of type string but was %T", value)
		}
		info.Announce = string(*l)
		return nil
	case "announce-list":
		l, ok := value.(*bencoding.List)
		if !ok {
			return fmt.Errorf("expected announce-list to be of type []ByteString but was %T", value)
		}

		for _, v := range *l {
			addr, ok := v.(*bencoding.ByteString)
			if !ok {
				return fmt.Errorf("expected address inside announce-list to be of type ByteString but was %T", value)
			}
			info.AnnounceList = append(info.AnnounceList, string(*addr))
		}
		return nil
	case "url-list":
		l, ok := value.(*bencoding.List)
		if !ok {
			return fmt.Errorf("expected url-list to be of type []ByteString but was %T", value)
		}

		for _, v := range *l {
			addr, ok := v.(*bencoding.ByteString)
			if !ok {
				return fmt.Errorf("expected address inside url-list to be of type ByteString but was %T", value)
			}
			info.UrlList = append(info.UrlList, string(*addr))
		}
		return nil
	case "creation date":
		l, ok := value.(*bencoding.Integer)
		if !ok {
			return fmt.Errorf("expected 'Creation Date' to be of type Interger but was %T", value)
		}
		t := time.Unix(int64(*l), 0)
		info.CreationDate = &t
		return nil
	case "comment":
		l, ok := value.(*bencoding.ByteString)
		if !ok {
			return fmt.Errorf("expected 'Commment' to be of type ByteString but was %T", value)
		}
		info.Comment = (*string)(l)
		return nil
	case "created by":
		l, ok := value.(*bencoding.ByteString)
		if !ok {
			return fmt.Errorf("expected 'Created By' to be of type ByteString but was %T", value)
		}
		info.CreatedBy = (*string)(l)
		return nil
	case "encoding":
		l, ok := value.(*bencoding.ByteString)
		if !ok {
			return fmt.Errorf("expected 'Encoding' to be of type ByteString but was %T", value)
		}
		info.Encoding = (*string)(l)
		return nil
	default:
		return fmt.Errorf("unsuported key: %s and its value: %s", key, value.Literal())
	}
}

func infoCommon(key string, value bencoding.Value, info *Info, isMultiFile bool) error {
	switch key {
	case "name":
		l, ok := value.(*bencoding.ByteString)
		if !ok {
			return fmt.Errorf("expected 'Name' to be of type ByteString but was %T", value)
		}

		if isMultiFile {
			if info.InfoMultiFile == nil {
				info.InfoMultiFile = &InfoMultiFile{}
			}
			info.InfoMultiFile.Name = string(*l)
		} else {
			if info.InfoSingleFile == nil {
				info.InfoSingleFile = &InfoSingleFile{}
			}
			info.InfoSingleFile.Name = string(*l)
		}
		return nil
	case "length":
		l, ok := value.(*bencoding.Integer)
		if !ok {
			return fmt.Errorf("expected 'Length' to be of type Integer but was %T", value)
		}
		if info.InfoSingleFile == nil {
			info.InfoSingleFile = &InfoSingleFile{}
		}
		info.InfoSingleFile.Length = int64(*l)
		return nil
	case "md5sum":
		l, ok := value.(*bencoding.ByteString)
		if !ok {
			return fmt.Errorf("expected 'Md5Sum' to be of type ByteString but was %T", value)
		}
		if info.InfoSingleFile == nil {
			info.InfoSingleFile = &InfoSingleFile{}
		}
		info.InfoSingleFile.Md5sum = (*string)(l)
		return nil
	case "files":
		l, ok := value.(*bencoding.List)
		if !ok {
			return fmt.Errorf("expected 'Files' to be of type List but was %T", value)
		}

		for _, dict := range *l {
			dict, ok := dict.(*bencoding.Dictionary)
			if !ok {
				return fmt.Errorf("expected item inside files to be of type Dictionary but was %T", value)
			}

			fi := FileInfo{}

			if l, ok := dict.Dict["length"]; ok {
				l, ok := l.(*bencoding.Integer)
				if !ok {
					return fmt.Errorf("expected 'Length' inside of 'Files' to be of type Integer but was %T", value)
				}
				fi.Length = int64(*l)
			}

			if s, ok := dict.Dict["md5sum"]; ok {
				s, ok := s.(*bencoding.ByteString)
				if !ok {
					return fmt.Errorf("expected 'Md5Sum' inside of 'Files' to be of type ByteString but was %T", value)
				}
				fi.Md5Sum = (*string)(s)
			}

			if p, ok := dict.Dict["path"]; ok {
				p, ok := p.(*bencoding.List)
				if !ok {
					return fmt.Errorf("expected 'Path' inside of 'Files' to be of type List but was %T", value)
				}

				var path string
				for _, v := range *p {
					p, ok := v.(*bencoding.ByteString)
					if !ok {
						return fmt.Errorf("expected item inside list 'Path' inside of 'Files' to be of type ByteString but was %T", value)
					}
					path = filepath.Join(path, string(*p))
				}
				fi.Path = path
			}
		}
		return nil
	// common field
	case "piece length":
		l, ok := value.(*bencoding.Integer)
		if !ok {
			return fmt.Errorf("expected 'Piece Length' to be of type Integer but was %T", value)
		}
		info.PieceLength = int64(*l)
		return nil
	case "pieces":
		l, ok := value.(*bencoding.ByteString)
		if !ok {
			return fmt.Errorf("expected 'Pieces' to be of type ByteString but was %T", value)
		}
		info.Pieces = hex.EncodeToString([]byte((*l)))
		return nil
	case "private":
		l, ok := value.(*bencoding.Integer)
		if !ok {
			return fmt.Errorf("expected 'Private' to be of type Integer but was %T", value)
		}
		info.Private = (*int64)(l)
		return nil
	default:
		return fmt.Errorf("unsupported key: %s and its value: %s", key, value.Literal())
	}
}

func validate(i *MetaInfoFile) error {
	if i.Announce == "" {
		return errors.New("unspecified 'announce' in torrent file")
	}
	if i.InfoSingleFile == nil && i.InfoMultiFile == nil {
		return errors.New("neither single file nor multi file mode specified")
	}
	if len(i.Info.Pieces) == 0 {
		return errors.New("missing 'pieces' inside torrent file")
	}
	h, err := hex.DecodeString(i.Info.Pieces)
	if err != nil {
		return err
	}
	if len(h)%20 != 0 {
		return errors.New("invalid 'pieces' value inside torrent file")
	}
	if i.InfoSingleFile != nil {
		if i.InfoSingleFile.Name == "" {
			return errors.New("missing 'name' for single file torrent")
		}
		if i.InfoSingleFile.Length == 0 {
			return errors.New("missing 'length' for single file torrent")
		}
	}
	if i.InfoMultiFile != nil {
		if i.InfoMultiFile.Name == "" {
			return errors.New("missing directory 'name' for multi file torrent")
		}
		for _, f := range i.InfoMultiFile.Files {
			if f.Length == 0 {
				return fmt.Errorf("missing 'length' inside %s for multi file torrent", i.InfoMultiFile.Name)
			}
			if len(f.Path) == 0 {
				return fmt.Errorf("missing 'Path' inside %s for multi file torrent", i.InfoMultiFile.Name)
			}
		}
	}
	return nil
}
