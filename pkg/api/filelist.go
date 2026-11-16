package api

import (
	"encoding/json"
	"errors"
	"fmt"
)

// MaxFileList bounds the files one FileList carries.
const MaxFileList = 2000

// ErrInvalidFileList is returned by FileListRequest.Validate.
var ErrInvalidFileList = errors.New("api: invalid file list request")

// FileListRequest asks the server for the files in a chat pane's folder.
type FileListRequest struct {
	Pane int `json:"pane"`
}

// Validate checks that r names a pane.
func (r *FileListRequest) Validate() error {
	if r.Pane <= 0 {
		return fmt.Errorf("%w: no pane", ErrInvalidFileList)
	}
	return nil
}

// FileList is the server's answer to FileListRequest. Pane and Folder
// repeat the request's chat folder, so a client can drop an answer to a
// question it no longer asks.
type FileList struct {
	Pane   int      `json:"pane"`
	Folder string   `json:"folder,omitempty"`
	Files  []string `json:"files,omitempty"` // slash paths from the folder, in walk order
	Cut    bool     `json:"cut,omitempty"`   // a file past MaxFileList was left out
	Err    string   `json:"err,omitempty"`   // why the folder could not be read
}

// MarshalJSON writes Files as [] when it is empty.
func (l FileList) MarshalJSON() ([]byte, error) {
	type wire struct {
		Pane   int      `json:"pane"`
		Folder string   `json:"folder,omitempty"`
		Files  []string `json:"files"`
		Cut    bool     `json:"cut,omitempty"`
		Err    string   `json:"err,omitempty"`
	}
	files := l.Files
	if files == nil {
		files = []string{}
	}
	return json.Marshal(wire{Pane: l.Pane, Folder: l.Folder, Files: files, Cut: l.Cut, Err: l.Err})
}

// Type implements Message.
func (*FileListRequest) Type() MsgType { return TypeFileListRequest }
func (*FileListRequest) message()      {}

// Type implements Message.
func (*FileList) Type() MsgType { return TypeFileList }
func (*FileList) message()      {}
