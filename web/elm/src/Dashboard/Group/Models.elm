module Dashboard.Group.Models exposing
    ( Card(..)
    , Pipeline
    , cardName
    , cardTeamName
    )

import Concourse exposing (JsonValue)
import Dict exposing (Dict)


type Card
    = PipelineCard Pipeline
    | InstanceGroupCard Pipeline (List Pipeline)


cardName : Card -> String
cardName c =
    case c of
        PipelineCard p ->
            p.name

        InstanceGroupCard p _ ->
            p.name


cardTeamName : Card -> String
cardTeamName c =
    case c of
        PipelineCard p ->
            p.teamName

        InstanceGroupCard p _ ->
            p.teamName


type alias Pipeline =
    { id : Int
    , name : String
    , instanceVars : Dict String JsonValue
    , teamName : String
    , public : Bool
    , isToggleLoading : Bool
    , isVisibilityLoading : Bool
    , paused : Bool
    , archived : Bool
    , stale : Bool
    , jobsDisabled : Bool
    }
