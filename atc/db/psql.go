package db

import sq "github.com/Masterminds/squirrel"

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
