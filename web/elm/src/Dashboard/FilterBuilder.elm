module Dashboard.FilterBuilder exposing (instanceGroupFilter)


quoted : String -> String
quoted s =
    "\"" ++ s ++ "\""



-- Note: this has to be in a separate module to avoid a long import cycle
-- as a result of this function being used in SideBar.InstanceGroup


instanceGroupFilter : { r | teamName : String, name : String } -> String
instanceGroupFilter { teamName, name } =
    "team:" ++ quoted teamName ++ " group:" ++ quoted name
