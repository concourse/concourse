package dbng

// base_resource_types: <- gced referenced by 0 workers
// | id | type | image | version |

// worker_resource_types: <- synced w/ worker creation
// | worker_name | base_resource_type_id |

// resource_caches: <- gced by cache collector
// | id | resource_cache_id | base_resource_type_id | source_hash | params_hash | version |

type WorkerResourceType struct {
	WorkerName string

	BaseResourceType
}

// func (wrt WorkerResourceType) Lookup(tx Tx) (int, bool, error) {
// 	var id int
// 	err := psql.Select("id").From("worker_resource_types").Where(sq.Eq{
// 		"worker_name": wrt.WorkerName,
// 		"type":        wrt.Type,
// 		"image":       wrt.Image,
// 		"version":     wrt.Version,
// 	}).RunWith(tx).QueryRow().Scan(&id)
// 	if err != nil {
// 		if err == sql.ErrNoRows {
// 			return 0, false, nil
// 		}

// 		return 0, false, err
// 	}

// 	return id, true, nil
// }

// var ErrWorkerResourceTypeAlreadyExists = errors.New("worker resource type already exists")

// func (wrt WorkerResourceType) Create(tx Tx) (int, error) {
// 	var id int
// 	err := psql.Insert("worker_resource_types").
// 		Columns(
// 			"worker_name",
// 			"type",
// 			"image",
// 			"version",
// 		).
// 		Values(
// 			wrt.WorkerName,
// 			wrt.Type,
// 			wrt.Image,
// 			wrt.Version,
// 		).
// 		Suffix("RETURNING id").
// 		RunWith(tx).
// 		QueryRow().
// 		Scan(&id)
// 	if err != nil {
// 		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "unique_violation" {
// 			return 0, ErrWorkerResourceTypeAlreadyExists
// 		}

// 		return 0, err
// 	}

// 	_, err = psql.Delete("worker_resource_types").
// 		Where(sq.And{
// 			sq.Eq{
// 				"worker_name": wrt.WorkerName,
// 				"type":        wrt.Type,
// 				"image":       wrt.Image,
// 			},
// 			sq.NotEq{
// 				"version": wrt.Version,
// 			},
// 		}).
// 		RunWith(tx).
// 		Exec()
// 	if err != nil {
// 		return 0, err
// 	}

// 	return id, nil
// }
