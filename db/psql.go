package db

import sq "github.com/Masterminds/squirrel"

const pqUniqueViolationErrCode = "unique_violation"
const pqFKeyViolationErrCode = "foreign_key_violation"

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
