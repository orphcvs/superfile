package preview

import (
	"log/slog"

	"github.com/yorukot/superfile/src/internal/common"

	filepreview "github.com/yorukot/superfile/src/pkg/file_preview"
)

type Model struct {
	open bool

	// Location denotes what is supposed to be in model.
	// Might not be always in sync with content
	location      string
	content       string
	contentWidth  int
	contentHeight int

	loading            bool
	imagePreviewer     *filepreview.ImagePreviewer
	batCmd             string
	thumbnailGenerator *filepreview.ThumbnailGenerator
	hasPdftotext       bool
	hasPandoc          bool
}

func New() Model {
	generator, err := filepreview.NewThumbnailGenerator()
	if err != nil {
		slog.Error("Could not NewThumbnailGenerator object", "error", err)
	}

	hasPdftotext, hasPandoc := checkDocTools()

	return Model{
		open: common.Config.DefaultOpenFilePreview,
		imagePreviewer:     filepreview.NewImagePreviewer(),
		thumbnailGenerator: generator,
		batCmd:             checkBatCmd(),
		hasPdftotext:       hasPdftotext,
		hasPandoc:          hasPandoc,
	}
}
