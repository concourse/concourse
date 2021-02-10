module Dashboard.Group.Models exposing
    ( Card(..)
    , Pipeline
    , cardIdentifier
    , cardName
    , cardTeamName
    )

import Concourse


type Card
    = PipelineCard Pipeline
    | InstancedPipelineCard Pipeline
    | InstanceGroupCard Pipeline (List Pipeline)


cardIdentifier : Card -> String
cardIdentifier c =
    case c of
        PipelineCard p ->
            String.fromInt p.id

        InstancedPipelineCard p ->
            String.fromInt p.id

        InstanceGroupCard p _ ->
            p.teamName ++ "/" ++ p.name


cardName : Card -> String
cardName c =
    case c of
        PipelineCard p ->
            p.name

        InstancedPipelineCard p ->
            p.name

        InstanceGroupCard p _ ->
            p.name


cardTeamName : Card -> String
cardTeamName c =
    case c of
        PipelineCard p ->
            p.teamName

        InstancedPipelineCard p ->
            p.teamName

        InstanceGroupCard p _ ->
            p.teamName


type alias Pipeline =
    { id : Int
    , name : String
    , instanceVars : Concourse.InstanceVars
    , teamName : String
    , public : Bool
    , isToggleLoading : Bool
    , isVisibilityLoading : Bool
    , paused : Bool
    , archived : Bool
    , stale : Bool
    , jobsDisabled : Bool
    }
