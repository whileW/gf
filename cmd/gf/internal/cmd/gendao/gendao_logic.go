package gendao

import (
	"bytes"
	"context"
	"fmt"
	"github.com/gogf/gf/cmd/gf/v2/internal/consts"
	"github.com/gogf/gf/cmd/gf/v2/internal/utility/mlog"
	"github.com/gogf/gf/cmd/gf/v2/internal/utility/utils"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gfile"
	"github.com/gogf/gf/v2/text/gstr"
	"github.com/olekukonko/tablewriter"
	"strings"
)

func generateLogic(ctx context.Context, in CGenDaoInternalInput) {
	var (
		dirPathLogic = gfile.Join(in.Path, "logic")
		dirPathModel = gfile.Join(in.Path, "model")
	)
	for i := 0; i < len(in.TableNames); i++ {
		generateLogicSingle(ctx, generateLogicSingleInput{
			CGenDaoInternalInput: in,
			TableName:            in.TableNames[i],
			NewTableName:         in.NewTableNames[i],
			dirPathLogic:         dirPathLogic,
			dirPathModel:         dirPathModel,
		})
	}
}

type generateLogicSingleInput struct {
	CGenDaoInternalInput
	TableName    string // TableName specifies the table name of the table.
	NewTableName string // NewTableName specifies the prefix-stripped name of the table.
	dirPathLogic string
	dirPathModel string
}

func generateLogicSingle(ctx context.Context, in generateLogicSingleInput) {
	dirPathLogicSingle := gfile.Join(in.dirPathLogic, in.NewTableName)
	fieldMap, err := in.DB.TableFields(ctx, in.TableName)
	if err != nil {
		mlog.Fatalf(`fetching tables fields failed for table "%s": %+v`, in.TableName, err)
	}

	var (
		tableNameCamelCase      = gstr.CaseCamel(in.NewTableName)
		tableNameCamelLowerCase = gstr.CaseCamelLower(in.NewTableName)
		tableNameSnakeCase      = gstr.CaseSnake(in.NewTableName)
		importPrefix            = in.ImportPrefix
	)
	if importPrefix == "" {
		importPrefix = utils.GetImportPath(gfile.Join(in.Path))
	}

	fileName := gstr.Trim(tableNameSnakeCase, "-_.")

	generateLogicModelIndex(ctx, generateLogicIndexInput{
		dirPathLogicSingle:       dirPathLogicSingle,
		generateLogicSingleInput: in,
		TableNameCamelCase:       tableNameCamelCase,
		TableNameCamelLowerCase:  tableNameCamelLowerCase,
		ImportPrefix:             importPrefix,
		FileName:                 fileName,
		FieldMap:                 fieldMap,
	})

	generateLogicIndex(generateLogicIndexInput{
		dirPathLogicSingle:       dirPathLogicSingle,
		generateLogicSingleInput: in,
		TableNameCamelCase:       tableNameCamelCase,
		TableNameCamelLowerCase:  tableNameCamelLowerCase,
		ImportPrefix:             importPrefix,
		FileName:                 fileName,
		FieldMap:                 fieldMap,
	})

}

type generateLogicIndexInput struct {
	generateLogicSingleInput
	dirPathLogicSingle      string
	TableNameCamelCase      string
	TableNameCamelLowerCase string
	ImportPrefix            string
	FileName                string
	FieldMap                map[string]*gdb.TableField
}

func generateLogicIndex(in generateLogicIndexInput) {
	path := gfile.Join(in.dirPathLogicSingle, in.FileName+".go")
	if !gfile.Exists(path) {
		fieldArr := getCreateColumnNames(in.FieldMap)
		indexContent := gstr.ReplaceByMap(
			getTemplateFromPathOrDefault("", consts.TemplateGenLogicIndexContent),
			g.MapStrStr{
				tplVarImportPrefix:            in.ImportPrefix,
				tplVarTableName:               in.TableName,
				tplVarTableNameCamelCase:      in.TableNameCamelCase,
				tplVarTableNameCamelLowerCase: in.TableNameCamelLowerCase,
				tplVarColumnCreate:            generateCreateColumnNamesForLogic(fieldArr),
				tplVarPageListSearch:          "",
			})
		indexContent = replaceDefaultVar(in.CGenDaoInternalInput, indexContent)
		if err := gfile.PutContents(path, strings.TrimSpace(indexContent)); err != nil {
			mlog.Fatalf("writing content to '%s' failed: %v", path, err)
		} else {
			utils.GoFmt(path)
			mlog.Print("generated:", path)
		}
	}
}
func generateLogicModelIndex(ctx context.Context, in generateLogicIndexInput) {
	path := gfile.Join(in.dirPathModel, in.FileName+".go")
	if !gfile.Exists(path) {
		fieldArr := getCreateColumnNames(in.FieldMap)
		columnCreate, appendImports := generateCreateColumnNamesForModel(ctx, fieldArr, generateStructDefinitionInput{
			CGenDaoInternalInput: in.CGenDaoInternalInput,
			TableName:            in.TableName,
			StructName:           gstr.CaseCamel(in.NewTableName),
			FieldMap:             in.FieldMap,
			IsDo:                 false,
		})
		indexContent := gstr.ReplaceByMap(
			getTemplateFromPathOrDefault("", consts.TemplateGenModelIndexContent),
			g.MapStrStr{
				tplVarBaseModelImportPrefix:   utils.GetImportPath(gfile.Join("utility", "base_model")),
				tplVarImportPrefix:            in.ImportPrefix,
				tplVarTableName:               in.TableName,
				tplVarTableNameCamelCase:      in.TableNameCamelCase,
				tplVarTableNameCamelLowerCase: in.TableNameCamelLowerCase,
				tplVarColumnCreate:            columnCreate,
				tplVarPageListSearch:          "",
				tplVarPackageImports:          strings.Join(appendImports, "\n"),
			})
		indexContent = replaceDefaultVar(in.CGenDaoInternalInput, indexContent)
		if err := gfile.PutContents(path, strings.TrimSpace(indexContent)); err != nil {
			mlog.Fatalf("writing content to '%s' failed: %v", path, err)
		} else {
			utils.GoFmt(path)
			mlog.Print("generated:", path)
		}
	}
}

func getCreateColumnNames(fieldMap map[string]*gdb.TableField) []*gdb.TableField {
	fieldMap = excludeBaseColumn(fieldMap)
	names := sortFieldKeyForDao(fieldMap)
	var fields []*gdb.TableField
	for _, name := range names {
		fields = append(fields, fieldMap[name])
	}
	return fields
}
func generateCreateColumnNamesForLogic(fieldArr []*gdb.TableField) string {
	var (
		buffer = bytes.NewBuffer(nil)
		array  = make([][]string, len(fieldArr))
	)

	for index, field := range fieldArr {
		array[index] = []string{
			"            #" + gstr.CaseCamel(field.Name) + ":",
			fmt.Sprintf(" #in.%s,", gstr.CaseCamel(field.Name)),
		}
	}
	tw := tablewriter.NewWriter(buffer)
	tw.SetBorder(false)
	tw.SetRowLine(false)
	tw.SetAutoWrapText(false)
	tw.SetColumnSeparator("")
	tw.AppendBulk(array)
	tw.Render()
	namesContent := buffer.String()
	namesContent = gstr.Replace(namesContent, "  #", "")
	buffer.Reset()
	buffer.WriteString(namesContent)
	return buffer.String()
}
func generateCreateColumnNamesForModel(ctx context.Context, fieldArr []*gdb.TableField, in generateStructDefinitionInput) (string, []string) {
	var (
		buffer        = bytes.NewBuffer(nil)
		array         = make([][]string, len(fieldArr))
		appendImports []string
	)

	for index, field := range fieldArr {
		var imports string
		array[index], imports = generateStructFieldDefinition(ctx, field, in)
		if imports != "" {
			appendImports = append(appendImports, imports)
		}
	}
	tw := tablewriter.NewWriter(buffer)
	tw.SetBorder(false)
	tw.SetRowLine(false)
	tw.SetAutoWrapText(false)
	tw.SetColumnSeparator("")
	tw.AppendBulk(array)
	tw.Render()
	stContent := buffer.String()
	// Let's do this hack of table writer for indent!
	stContent = gstr.Replace(stContent, "  #", "")
	stContent = gstr.Replace(stContent, "` ", "`")
	stContent = gstr.Replace(stContent, "``", "")
	buffer.Reset()
	buffer.WriteString(stContent)
	return buffer.String(), appendImports
}

func excludeBaseColumn(fieldMap map[string]*gdb.TableField) map[string]*gdb.TableField {
	newFieldMap := make(map[string]*gdb.TableField)
	for k, v := range fieldMap {
		if !gstr.InArray([]string{"id", "created_at", "updated_at", "deleted_at"}, k) {
			newFieldMap[k] = v
		}
	}
	return newFieldMap
}
