package db2

import (
	"context"

	"github.com/Breeze0806/go-etl/datax/common/plugin"
	"github.com/Breeze0806/go-etl/datax/plugin/writer/rdbm"
	"github.com/Breeze0806/go-etl/storage/database"

	//db2 dialect
	_ "github.com/Breeze0806/go-etl/storage/database/db2"
)

var execModeMap = map[string]string{
	database.WriteModeInsert: rdbm.ExecModeNormal,
}

func execMode(writeMode string) string {
	if mode, ok := execModeMap[writeMode]; ok {
		return mode
	}
	return rdbm.ExecModeNormal
}

//Task 任务
type Task struct {
	*rdbm.Task
}

//StartWrite 开始写
func (t *Task) StartWrite(ctx context.Context, receiver plugin.RecordReceiver) (err error) {
	writer := rdbm.NewBaseBatchWriter(t.Task, execMode(t.Config.GetWriteMode()), nil)
	return rdbm.StartWrite(ctx, writer, receiver)
}