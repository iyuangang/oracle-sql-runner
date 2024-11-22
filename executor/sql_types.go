package executor

import "strings"

type SQLType int

const (
	SQLTypeUnknown SQLType = iota
	SQLTypeDDL
	SQLTypeDML
	SQLTypeDCL
	SQLTypeTCL
	SQLTypePLSQL
	SQLTypeAnonymousBlock
	SQLTypeQuery
)

func (st SQLType) String() string {
	switch st {
	case SQLTypeDDL:
		return "DDL"
	case SQLTypeDML:
		return "DML"
	case SQLTypeDCL:
		return "DCL"
	case SQLTypeTCL:
		return "TCL"
	case SQLTypePLSQL:
		return "PL/SQL"
	case SQLTypeAnonymousBlock:
		return "Anonymous Block"
	case SQLTypeQuery:
		return "Query"
	default:
		return "Unknown"
	}
}

func DetermineSQLType(stmt string) SQLType {
	stmt = strings.TrimSpace(strings.ToUpper(stmt))

	// DDL语句
	if strings.HasPrefix(stmt, "CREATE ") ||
		strings.HasPrefix(stmt, "ALTER ") ||
		strings.HasPrefix(stmt, "DROP ") ||
		strings.HasPrefix(stmt, "TRUNCATE ") {
		return SQLTypeDDL
	}

	// DML语句
	if strings.HasPrefix(stmt, "INSERT ") ||
		strings.HasPrefix(stmt, "UPDATE ") ||
		strings.HasPrefix(stmt, "DELETE ") ||
		strings.HasPrefix(stmt, "MERGE ") {
		return SQLTypeDML
	}

	// DCL语句
	if strings.HasPrefix(stmt, "GRANT ") ||
		strings.HasPrefix(stmt, "REVOKE ") {
		return SQLTypeDCL
	}

	// TCL语句
	if strings.HasPrefix(stmt, "COMMIT") ||
		strings.HasPrefix(stmt, "ROLLBACK") ||
		strings.HasPrefix(stmt, "SAVEPOINT") {
		return SQLTypeTCL
	}

	// PL/SQL块
	if strings.Contains(stmt, "CREATE OR REPLACE") &&
		(strings.Contains(stmt, "PROCEDURE") ||
			strings.Contains(stmt, "FUNCTION") ||
			strings.Contains(stmt, "PACKAGE") ||
			strings.Contains(stmt, "TRIGGER")) {
		return SQLTypePLSQL
	}

	// 匿名块
	if strings.HasPrefix(stmt, "DECLARE") ||
		strings.HasPrefix(stmt, "BEGIN") {
		return SQLTypeAnonymousBlock
	}

	// 查询语句
	if strings.HasPrefix(stmt, "SELECT ") {
		return SQLTypeQuery
	}

	return SQLTypeUnknown
}
