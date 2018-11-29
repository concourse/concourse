port module Dashboard.Group exposing (..)

import Concourse
import Concourse.Info
import Concourse.Job
import Concourse.Pipeline
import Concourse.PipelineStatus
import Concourse.Resource
import Concourse.Team
import Dashboard.APIData exposing (APIData)
import Dashboard.Msgs exposing (Msg(..))
import Dashboard.Pipeline as Pipeline
import Dashboard.Styles as Styles
import Html exposing (..)
import Html.Attributes exposing (..)
import Html.Events exposing (on, onMouseEnter)
import Http
import Json.Decode
import List.Extra
import Maybe.Extra
import Monocle.Iso
import Monocle.Optional
import Monocle.Lens
import NewTopBar.Styles as NTBS
import Ordering exposing (Ordering)
import Set
import Task
import Time exposing (Time)


type alias Group =
    { pipelines : List Pipeline.PipelineWithJobs
    , teamName : String
    }


type alias StickyHeaderConfig =
    { pageHeaderHeight : Float
    , pageBodyClass : String
    , sectionHeaderClass : String
    , sectionClass : String
    , sectionBodyClass : String
    }


port pinTeamNames : StickyHeaderConfig -> Cmd msg


stickyHeaderConfig : StickyHeaderConfig
stickyHeaderConfig =
    { pageHeaderHeight = NTBS.pageHeaderHeight
    , pageBodyClass = "dashboard"
    , sectionClass = "dashboard-team-group"
    , sectionHeaderClass = "dashboard-team-header"
    , sectionBodyClass = "dashboard-team-pipelines"
    }


groupsLens : Monocle.Lens.Lens APIData (List Group)
groupsLens =
    Monocle.Lens.fromIso <| Monocle.Iso.Iso groups apiData


findGroupOptional : String -> Monocle.Optional.Optional (List Group) Group
findGroupOptional teamName =
    let
        predicate =
            .teamName >> (==) teamName
    in
        Monocle.Optional.Optional (List.Extra.find predicate)
            (\g gs -> List.Extra.findIndex predicate gs |> Maybe.map (\i -> List.Extra.setAt i g gs) |> Maybe.Extra.join |> Maybe.withDefault gs)


type alias PipelineIndex =
    Int


type DragState
    = NotDragging
    | Dragging Concourse.TeamName PipelineIndex


teamNameOptional : Monocle.Optional.Optional DragState Concourse.TeamName
teamNameOptional =
    Monocle.Optional.Optional teamName setTeamName


dragIndexOptional : Monocle.Optional.Optional DragState PipelineIndex
dragIndexOptional =
    Monocle.Optional.Optional dragIndex setDragIndex


dropIndexOptional : Monocle.Optional.Optional DropState PipelineIndex
dropIndexOptional =
    Monocle.Optional.Optional dropIndex setDropIndex


teamName : DragState -> Maybe Concourse.TeamName
teamName dragState =
    case dragState of
        Dragging teamName _ ->
            Just teamName

        NotDragging ->
            Nothing


setTeamName : Concourse.TeamName -> DragState -> DragState
setTeamName teamName dragState =
    case dragState of
        Dragging _ dragIndex ->
            Dragging teamName dragIndex

        NotDragging ->
            NotDragging


dragIndex : DragState -> Maybe PipelineIndex
dragIndex dragState =
    case dragState of
        Dragging _ dragIndex ->
            Just dragIndex

        NotDragging ->
            Nothing


setDragIndex : PipelineIndex -> DragState -> DragState
setDragIndex dragIndex dragState =
    case dragState of
        Dragging teamName _ ->
            Dragging teamName dragIndex

        NotDragging ->
            NotDragging


type DropState
    = NotDropping
    | Dropping PipelineIndex


dropIndex : DropState -> Maybe PipelineIndex
dropIndex dropState =
    case dropState of
        Dropping dropIndex ->
            Just dropIndex

        NotDropping ->
            Nothing


setDropIndex : PipelineIndex -> DropState -> DropState
setDropIndex dropIndex dropState =
    case dropState of
        Dropping _ ->
            Dropping dropIndex

        NotDropping ->
            NotDropping


allPipelines : APIData -> List Pipeline.PipelineWithJobs
allPipelines data =
    data.pipelines
        |> List.map
            (\p ->
                { pipeline = p
                , jobs =
                    data.jobs
                        |> List.filter
                            (\j ->
                                (j.teamName == p.teamName)
                                    && (j.pipelineName == p.name)
                            )
                , resourceError =
                    data.resources
                        |> List.any
                            (\r ->
                                (r.teamName == p.teamName)
                                    && (r.pipelineName == p.name)
                                    && r.failingToCheck
                            )
                }
            )


shiftPipelines : Int -> Int -> Group -> Group
shiftPipelines dragIndex dropIndex group =
    if dragIndex == dropIndex then
        group
    else
        let
            pipelines =
                case
                    List.head <|
                        List.drop dragIndex <|
                            group.pipelines
                of
                    Nothing ->
                        group.pipelines

                    Just pipeline ->
                        shiftPipelineTo pipeline dropIndex group.pipelines
        in
            { group | pipelines = pipelines }



-- TODO this is pretty hard to reason about. really deeply nested and nasty. doesn't exactly relate
-- to the hd refactor as hd doesn't have the drag-and-drop feature, but it's a big contributor
-- to the 'length of this file' tire fire


shiftPipelineTo : Pipeline.PipelineWithJobs -> Int -> List Pipeline.PipelineWithJobs -> List Pipeline.PipelineWithJobs
shiftPipelineTo ({ pipeline } as pipelineWithJobs) position pipelines =
    case pipelines of
        [] ->
            if position < 0 then
                []
            else
                [ pipelineWithJobs ]

        p :: ps ->
            if p.pipeline.teamName /= pipeline.teamName then
                p :: shiftPipelineTo pipelineWithJobs position ps
            else if p.pipeline == pipeline then
                shiftPipelineTo pipelineWithJobs (position - 1) ps
            else if position == 0 then
                pipelineWithJobs :: p :: shiftPipelineTo pipelineWithJobs (position - 1) ps
            else
                p :: shiftPipelineTo pipelineWithJobs (position - 1) ps


allTeamNames : APIData -> List String
allTeamNames apiData =
    Set.union
        (Set.fromList (List.map .teamName apiData.pipelines))
        (Set.fromList (List.map .name apiData.teams))
        |> Set.toList


remoteData : Task.Task Http.Error APIData
remoteData =
    Task.map5 APIData
        Concourse.Team.fetchTeams
        Concourse.Pipeline.fetchPipelines
        (Concourse.Job.fetchAllJobs |> Task.map (Maybe.withDefault []))
        (Concourse.Resource.fetchAllResources |> Task.map (Maybe.withDefault []))
        (Concourse.Info.fetch |> Task.map .version)


groups : APIData -> List Group
groups apiData =
    let
        teamNames =
            allTeamNames apiData
    in
        teamNames
            |> List.map (group (allPipelines apiData))



-- TODO i'd like for this to be an isomorphism, which would
-- require adding resource data to the Group type, or making
-- the APIData type smaller (or, like, not marrying Group to
-- APIData at all but using a different type)


apiData : List Group -> APIData
apiData groups =
    let
        pipelines =
            groups |> List.concatMap .pipelines
    in
        { teams = groups |> List.map (\g -> { id = 0, name = g.teamName })
        , pipelines = pipelines |> List.map .pipeline
        , jobs = pipelines |> List.concatMap .jobs
        , resources = []
        , version = ""
        }


group : List Pipeline.PipelineWithJobs -> String -> Group
group allPipelines teamName =
    { pipelines = (List.filter ((==) teamName << .teamName << .pipeline) allPipelines)
    , teamName = teamName
    }


ordering : Ordering Group
ordering =
    Ordering.byField .teamName


view :
    { header : List (Html Msg)
    , dragState : DragState
    , dropState : DropState
    , now : Time
    , hoveredPipeline : Maybe Concourse.Pipeline
    }
    -> Group
    -> Html Msg
view { header, dragState, dropState, now, hoveredPipeline } group =
    let
        pipelines =
            if List.isEmpty group.pipelines then
                [ Pipeline.pipelineNotSetView ]
            else
                List.append
                    (List.indexedMap
                        (\i pipeline ->
                            let
                                pipelineStatus =
                                    Pipeline.pipelineStatusFromJobs pipeline.jobs False

                                bannerStatus =
                                    if pipeline.pipeline.paused then
                                        Concourse.PipelineStatusPaused
                                    else
                                        Pipeline.pipelineStatusFromJobs pipeline.jobs False

                                running =
                                    not <|
                                        List.isEmpty <|
                                            List.filterMap .nextBuild pipeline.jobs
                            in
                                Html.div [ class "pipeline-wrapper" ]
                                    [ pipelineDropAreaView dragState dropState group.teamName i
                                    , Html.div
                                        [ classList
                                            [ ( "dashboard-pipeline", True )
                                            , ( "dashboard-paused", pipeline.pipeline.paused )
                                            , ( "dashboard-running", running )
                                            , ( "dashboard-status-"
                                                    ++ Concourse.PipelineStatus.show pipelineStatus
                                              , not pipeline.pipeline.paused
                                              )
                                            , ( "dragging"
                                              , dragState == Dragging pipeline.pipeline.teamName i
                                              )
                                            ]
                                        , attribute "data-pipeline-name" pipeline.pipeline.name
                                        , attribute
                                            "ondragstart"
                                            "event.dataTransfer.setData('text/plain', '');"
                                        , draggable "true"
                                        , on "dragstart"
                                            (Json.Decode.succeed (DragStart pipeline.pipeline.teamName i))
                                        , on "dragend" (Json.Decode.succeed DragEnd)
                                        ]
                                        [ Html.div
                                            [ class "dashboard-pipeline-banner"
                                            , style <|
                                                Styles.pipelineCardBanner
                                                    { status = bannerStatus
                                                    , running = running
                                                    }
                                            ]
                                            []
                                        , Pipeline.pipelineView
                                            { now = now
                                            , pipelineWithJobs = pipeline
                                            , hovered = hoveredPipeline == Just pipeline.pipeline
                                            }
                                        ]
                                    ]
                        )
                        group.pipelines
                    )
                    [ pipelineDropAreaView dragState dropState group.teamName (List.length group.pipelines) ]
    in
        Html.div
            [ id group.teamName
            , class "dashboard-team-group"
            , attribute "data-team-name" group.teamName
            ]
            [ Html.div
                [ style [ ( "display", "flex" ), ( "align-items", "center" ) ]
                , class stickyHeaderConfig.sectionHeaderClass
                ]
                header
            , Html.div [ class stickyHeaderConfig.sectionBodyClass ] pipelines
            ]


hdView : List (Html Msg) -> String -> List Pipeline.PipelineWithJobs -> Html Msg
hdView header teamName pipelines =
    let
        teamPipelines =
            if List.isEmpty pipelines then
                [ pipelineNotSetView ]
            else
                List.map Pipeline.hdPipelineView pipelines
    in
        Html.div [ class "pipeline-wrapper" ] <|
            case teamPipelines of
                [] ->
                    header

                p :: ps ->
                    -- Wrap the team name and the first pipeline together so the team name is not the last element in a column
                    List.append [ Html.div [ class "dashboard-team-name-wrapper" ] (header ++ [ p ]) ] ps


pipelineNotSetView : Html msg
pipelineNotSetView =
    Html.div
        [ class "dashboard-pipeline" ]
        [ Html.div
            [ classList
                [ ( "dashboard-pipeline-content", True )
                , ( "no-set", True )
                ]
            ]
            [ Html.a [] [ Html.text "no pipelines set" ]
            ]
        ]


headerView : Group -> List (Html Msg)
headerView group =
    [ Html.div [ class "dashboard-team-name" ] [ Html.text group.teamName ] ]


pipelineDropAreaView : DragState -> DropState -> String -> Int -> Html Msg
pipelineDropAreaView dragState dropState teamName index =
    let
        ( active, over ) =
            case ( dragState, dropState ) of
                ( Dragging team dragIndex, NotDropping ) ->
                    ( team == teamName, index == dragIndex )

                ( Dragging team dragIndex, Dropping dropIndex ) ->
                    ( team == teamName, index == dropIndex )

                _ ->
                    ( False, False )
    in
        Html.div
            [ classList [ ( "drop-area", True ), ( "active", active ), ( "over", over ), ( "animation", dropState /= NotDropping ) ]
            , on "dragenter" (Json.Decode.succeed (DragOver teamName index))
            ]
            [ Html.text "" ]
