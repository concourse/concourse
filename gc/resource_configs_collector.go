package gc

// * remove resource_configs_uses:
//    * if its resource or resource type is NULL (or not active)
// 	  * if its build is completed and it is not latest failed build

// * remove resource_configs that have no uses
