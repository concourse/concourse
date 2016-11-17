package gc

// * remove resource_configs_uses:
//    * if its resource or resource type is NULL (or not active) - and it wasn't always (i.e. build is also null)
// 	  * if its build is completed and it is not latest failed build - and its build is not null

// * remove resource_configs that have no uses
