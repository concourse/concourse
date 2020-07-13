module Dashboard.Group.Models exposing (Group, Pipeline)


type alias Group =
    { pipelines : List Pipeline
    , teamName : String
    }


type alias Pipeline =
    { id : Int
    , name : String
    , teamName : String
    , public : Bool
    , isToggleLoading : Bool
    , isVisibilityLoading : Bool
    , paused : Bool
    , archived : Bool
    , stale : Bool
    , jobsDisabled : Bool
    , isFavorited : Bool
    }
