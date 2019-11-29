module Dashboard.Group.Models exposing (Group, Pipeline)


type alias Group =
    { pipelines : List Pipeline
    , teamName : String
    }


type alias Pipeline =
    { id : Int
    , ordering : Int
    , name : String
    , teamName : String
    , public : Bool
    , isToggleLoading : Bool
    , isVisibilityLoading : Bool
    , paused : Bool
    }
