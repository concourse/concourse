package gc

// worker_base_resource_types:
// | worker_name | base_resource_type_id |

// DELETE FROM base_resource_types WHERE id NOT IN (SELECT DISTINCT base_resource_type_id FROM worker_based_resource_tyoes)
