module Dashboard.Group.Models exposing
    ( Card(..)
    , Pipeline
    , cardIdentifier
    , cardName
    , cardTeamName
    , groupCardsWithinTeam
    , ungroupCards
    )

import Concourse


type Card
    = PipelineCard Pipeline
    | InstancedPipelineCard Pipeline
    | InstanceGroupCard Pipeline (List Pipeline)


groupCardsWithinTeam : List Pipeline -> List Card
groupCardsWithinTeam =
    Concourse.groupPipelinesWithinTeam
        >> List.map
            (\g ->
                case g of
                    Concourse.RegularPipeline p ->
                        PipelineCard p

                    Concourse.InstanceGroup p ps ->
                        InstanceGroupCard p ps
            )


ungroupCards : List Card -> List Pipeline
ungroupCards =
    List.concatMap
        (\c ->
            case c of
                PipelineCard p ->
                    [ p ]

                InstancedPipelineCard p ->
                    [ p ]

                InstanceGroupCard p ps ->
                    p :: ps
        )


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
