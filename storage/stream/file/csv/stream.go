// Copyright 2020 the go-etl Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package csv

import (
	"encoding/csv"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/Breeze0806/go-etl/config"
	"github.com/Breeze0806/go-etl/element"
	"github.com/Breeze0806/go-etl/storage/stream/file"
	"github.com/pingcap/errors"
)

func init() {
	var opener Opener
	file.RegisterOpener("csv", &opener)
	var creator Creator
	file.RegisterCreator("csv", &creator)
}

//Opener csv输入流打开器
type Opener struct {
}

//Open 打开一个名为filename的csv输入流
func (o *Opener) Open(filename string) (file.InStream, error) {
	return NewInStream(filename)
}

//Creator csv输出流创建器
type Creator struct {
}

//Create 创建一个名为filename的csv输出流
func (c *Creator) Create(filename string) (file.OutStream, error) {
	return NewOutStream(filename)
}

//Stream csv文件流
type Stream struct {
	file *os.File
}

//NewInStream 创建一个名为filename的csv输入流
func NewInStream(filename string) (file.InStream, error) {
	stream := &Stream{}
	var err error
	stream.file, err = os.Open(filename)
	if err != nil {
		return nil, err
	}
	return stream, nil
}

//NewOutStream 创建一个名为filename的csv输出流
func NewOutStream(filename string) (file.OutStream, error) {
	stream := &Stream{}
	var err error
	stream.file, err = os.Create(filename)
	if err != nil {
		return nil, err
	}
	return stream, nil
}

//Writer 新建一个配置未conf的csv流写入器
func (s *Stream) Writer(conf *config.JSON) (file.StreamWriter, error) {
	return NewWriter(s.file, conf)
}

//Rows 新建一个配置未conf的csv行读取器
func (s *Stream) Rows(conf *config.JSON) (rows file.Rows, err error) {
	return NewRows(s.file, conf)
}

//Close 关闭文件流
func (s *Stream) Close() (err error) {
	return s.file.Close()
}

//Rows 行读取器
type Rows struct {
	columns    map[int]Column
	reader     *csv.Reader
	record     []string
	nullFormat string
	err        error
}

//NewRows 通过文件句柄f，和配置文件c 创建行读取器
func NewRows(f *os.File, c *config.JSON) (file.Rows, error) {
	var conf *Config
	var err error
	if conf, err = NewConfig(c); err != nil {
		return nil, err
	}
	rows := &Rows{
		columns:    make(map[int]Column),
		nullFormat: conf.NullFormat,
	}
	rows.reader = csv.NewReader(f)
	rows.reader.Comma = []rune(conf.Delimiter)[0]
	for _, v := range conf.Columns {
		rows.columns[v.index()] = v
	}
	return rows, nil
}

// Next 是否有下一行
func (r *Rows) Next() bool {
	if r.record, r.err = r.reader.Read(); r.err != nil {
		if r.err == io.EOF {
			r.err = nil
		}
		return false
	}
	return true
}

// Scan 扫描成列
func (r *Rows) Scan() (columns []element.Column, err error) {
	for i, v := range r.record {
		var c element.Column
		c, err = r.getColum(i, v)
		if err != nil {
			return nil, err
		}
		columns = append(columns, c)
	}
	return
}

// Error 读取中的错误
func (r *Rows) Error() error {
	return r.err
}

// Close 关闭读文件流
func (r *Rows) Close() error {
	return nil
}

func (r *Rows) getColum(index int, s string) (element.Column, error) {
	c, ok := r.columns[index]
	if ok && element.ColumnType(c.Type) == element.TypeTime {
		if s == r.nullFormat {
			return element.NewDefaultColumn(element.NewNilTimeColumnValue(),
				strconv.Itoa(index), 0), nil
		}
		layout := c.layout()
		t, err := time.Parse(layout, s)
		if err != nil {
			return nil, errors.Wrapf(err, "Parse time fail. layout: %v", layout)
		}
		return element.NewDefaultColumn(element.NewTimeColumnValueWithDecoder(t,
			element.NewStringTimeDecoder(layout)),
			strconv.Itoa(index), 0), nil
	}
	if s == r.nullFormat {
		return element.NewDefaultColumn(element.NewNilStringColumnValue(),
			strconv.Itoa(index), 0), nil
	}
	return element.NewDefaultColumn(element.NewStringColumnValue(s), strconv.Itoa(index), 0), nil
}

//Writer csv流写入器
type Writer struct {
	writer     *csv.Writer
	columns    map[int]Column
	nullFormat string
}

//NewWriter 通过文件句柄f，和配置文件c 创建csv流写入器
func NewWriter(f *os.File, c *config.JSON) (file.StreamWriter, error) {
	var conf *Config
	var err error
	if conf, err = NewConfig(c); err != nil {
		return nil, err
	}
	w := &Writer{
		writer:     csv.NewWriter(f),
		columns:    make(map[int]Column),
		nullFormat: conf.NullFormat,
	}
	w.writer.Comma = []rune(conf.Delimiter)[0]
	for _, v := range conf.Columns {
		w.columns[v.index()] = v
	}
	return w, nil
}

//Flush 刷新至磁盘
func (w *Writer) Flush() (err error) {
	w.writer.Flush()
	return
}

//Close 关闭
func (w *Writer) Close() (err error) {
	w.writer.Flush()
	return
}

//Write 将记录record 写入csv文件
func (w *Writer) Write(record element.Record) (err error) {
	var records []string
	for i := 0; i < record.ColumnNumber(); i++ {
		var col element.Column
		if col, err = record.GetByIndex(i); err != nil {
			return
		}
		var s string
		if s, err = w.getRecord(col, i); err != nil {
			return
		}
		records = append(records, s)
	}
	return w.writer.Write(records)
}

func (w *Writer) getRecord(col element.Column, i int) (s string, err error) {
	if col.IsNil() {
		return w.nullFormat, nil
	}

	if c, ok := w.columns[i]; ok && element.ColumnType(c.Type) == element.TypeTime {
		var t time.Time
		if t, err = col.AsTime(); err != nil {
			return
		}
		s = t.Format(c.layout())
		return
	}
	s, err = col.AsString()
	return
}
