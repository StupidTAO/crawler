package sqlstorage

import (
	"encoding/json"
	"github.com/StupidTAO/crawler/collector"
	"github.com/StupidTAO/crawler/engine"
	"github.com/StupidTAO/crawler/sqldb"
	"go.uber.org/zap"
)

type SqlStore struct {
	dataDocker  []*collector.DataCell //分批输出结果缓存
	columnNames []sqldb.Field         //标题字段
	db          sqldb.DBer
	Table       map[string]struct{}
	options
}

func New(opts ...Option) (*SqlStore, error) {
	options := defaultOptions
	for _, opt := range opts {
		opt(&options)
	}
	s := &SqlStore{}
	s.options = options
	s.Table = make(map[string]struct{})
	var err error
	s.db, err = sqldb.New(
		sqldb.WithLogger(s.logger),
		sqldb.WithConnUrl(s.sqlUrl),
	)
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (s *SqlStore) Save(dataCells ...*collector.DataCell) error {
	for _, cell := range dataCells {
		name := cell.GetTableName()
		if _, ok := s.Table[name]; !ok {
			columnNames := getFields(cell)

			//创建表
			err := s.db.CreateTable(sqldb.TableData{
				TableName:   name,
				ColumnNames: columnNames,
				AutoKey:     true,
			})
			if err != nil {
				s.logger.Error("create table failed", zap.Error(err))
			}
			s.Table[name] = struct{}{}
		}

		s.dataDocker = append(s.dataDocker, cell)
		if len(s.dataDocker) >= s.BatchCount {
			err := s.Flush()
			if err != nil {
				s.logger.Error("mysql flush db is failed!", zap.Error(err))
				continue
			}
		}
	}
	return nil
}

func getFields(cell *collector.DataCell) []sqldb.Field {
	taskName := cell.Data["Task"].(string)
	ruleName := cell.Data["Rule"].(string)
	fields := engine.GetFields(taskName, ruleName)

	var columnNames []sqldb.Field
	for _, field := range fields {
		columnNames = append(columnNames, sqldb.Field{
			Title: field,
			Type:  "MEDIUMTEXT",
		})
	}
	columnNames = append(columnNames,
		sqldb.Field{Title: "Url", Type: "VARCHAR(255)"},
		sqldb.Field{Title: "Time", Type: "VARCHAR(255)"})

	return columnNames
}

func (s *SqlStore) Flush() error {
	if len(s.dataDocker) == 0 {
		return nil
	}
	args := make([]interface{}, 0)
	for _, datacell := range s.dataDocker {
		ruleName := datacell.Data["Rule"].(string)
		taskName := datacell.Data["Task"].(string)
		fields := engine.GetFields(taskName, ruleName)
		data := datacell.Data["Data"].(map[string]interface{})
		value := []string{}
		for _, field := range fields {
			v := data[field]
			switch v.(type) {
			case nil:
				value = append(value, "")
			case string:
				value = append(value, v.(string))
			default:
				j, err := json.Marshal(v)
				if err != nil {
					value = append(value, "")
				} else {
					value = append(value, string(j))
				}
			}
		}
		value = append(value, datacell.Data["Url"].(string), datacell.Data["Time"].(string))
		for _, v := range value {
			args = append(args, v)
		}
	}

	err := s.db.Insert(sqldb.TableData{
		TableName:   s.dataDocker[0].GetTableName(),
		ColumnNames: getFields(s.dataDocker[0]),
		Args:        args,
		DataCount:   len(s.dataDocker),
	})
	if err != nil {
		s.logger.Error("flush data failed!", zap.Error(err))
		return err
	}
	//清空dataDocker中的数据
	s.dataDocker = []*collector.DataCell{}
	return nil
}
