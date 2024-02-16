package csv

import (
	"context"
	"encoding/csv"
	"io"
	"os"
	"strings"

	"github.com/layer5io/meshkit/utils"
)

type CSV[E any] struct {
	Context      context.Context
	cancel       context.CancelFunc
	reader       *csv.Reader
	filePath     string
	lineForColNo int
	// Stores the mapping for coumn name to golang equivalent attribute name.
	// It is optional and default mapping is the lower case representation with spaces replaced with "_"
	// eg: ColumnnName: Descritption, equivalent golang attribute to which it will be mapped during unmarshal "description".
	columnToNameMapping map[string]string
	predicateFunc       func(columns []string, currentRow []string) bool
}

func NewCSVParser[E any](filePath string, lineForColNo int, colToNameMapping map[string]string, predicateFunc func(columns []string, currentRow []string) bool) (*CSV[E], error) {
	reader, err := os.Open(filePath)
	if err != nil {
		return nil, utils.ErrReadFile(err, filePath)
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &CSV[E]{
		Context:             ctx,
		cancel:              cancel,
		reader:              csv.NewReader(reader),
		filePath:            filePath,
		columnToNameMapping: colToNameMapping,
		lineForColNo:        lineForColNo,
		predicateFunc:       predicateFunc,
	}, nil
}

// "lineForColNo" line number where the columns are defined in the csv
func (c *CSV[E]) ExtractCols(lineForColNo int) ([]string, error) {
	data := []string{}
	var err error
	for i := 0; i <= lineForColNo; i++ {
		data, err = c.reader.Read()
		if err != nil {
			return nil, utils.ErrReadFile(err, c.filePath)
		}
	}
	return data, nil
}

func (c *CSV[E]) Parse(ch chan E, errorChan chan error) error {
	defer func() {
		c.cancel()
	}()

	columnNames, err := c.ExtractCols(c.lineForColNo)
	size := len(columnNames)
	if err != nil {
		return utils.ErrReadFile(err, c.filePath)
	}
	for {
		data := make(map[string]interface{})
		values, err := c.reader.Read()
		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		if c.predicateFunc != nil && c.predicateFunc(columnNames, values) {
			for index, value := range values {
				var attribute string
				if index < size {
					attribute = strings.ReplaceAll(strings.ToLower(columnNames[index]), " ", "_")
					if c.columnToNameMapping != nil {
						key, ok := c.columnToNameMapping[columnNames[index]]
						if ok {
							attribute = key
						}
					}
					data[attribute] = value
				}
			}

			parsedData, err := utils.MarshalAndUnmarshal[map[string]interface{}, E](data)
			if err != nil {
				errorChan <- err
				continue
			}
			ch <- parsedData
		}

	}
	return nil
}
