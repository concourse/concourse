module Dashboard.Group.Models exposing (Group, Pipeline)


import Concourse


type alias Group =
    { pipelines : List Pipeline
    , teamName : String
    }


type alias Pipeline =
    { id : Int
    , name : String
    , instanceVars : Maybe Concourse.InstanceVars
    , teamName : String
    , public : Bool
    , isToggleLoading : Bool
    , isVisibilityLoading : Bool
    , paused : Bool
    , archived : Bool
    , stale : Bool
    , jobsDisabled : Bool
    }
