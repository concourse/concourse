module Dashboard.Group exposing
    ( PipelineIndex
    , allPipelines
    , allTeamNames
    , dragIndex
    , dragIndexOptional
    , dropIndex
    , dropIndexOptional
    , findGroupOptional
    , group
    , groups
    , hdView
    , jobStatus
    , ordering
    , pipelineDropAreaView
    , pipelineNotSetView
    , pipelineStatus
    , setDragIndex
    , setDropIndex
    , setTeamName
    , shiftPipelineTo
    , shiftPipelines
    , teamName
    , teamNameOptional
    , transition
    , view
    )

import Concourse
import Concourse.BuildStatus
import Concourse.PipelineStatus as PipelineStatus
import Dashboard.Group.Models exposing (Group, Pipeline)
import Dashboard.Group.Tag as Tag
import Dashboard.Models exposing (DragState(..), DropState(..))
import Dashboard.Pipeline as Pipeline
import Dashboard.Styles as Styles
import Html exposing (Html)
import Html.Attributes exposing (attribute, class, classList, draggable, id, style)
import Html.Events exposing (on)
import Json.Decode
import List.Extra
import Maybe.Extra
import Message.Effects as Effects
import Message.Message exposing (DomID(..), Message(..))
import Monocle.Optional
import Ordering exposing (Ordering)
import Set
import Time
import UserState exposing (UserState)


ordering : Ordering Group
ordering =
    Ordering.byFieldWith Tag.ordering .tag
        |> Ordering.breakTiesWith (Ordering.byField .teamName)


findGroupOptional : String -> Monocle.Optional.Optional (List Group) Group
findGroupOptional name =
    let
        predicate =
            .teamName >> (==) name
    in
    Monocle.Optional.Optional (List.Extra.find predicate)
        (\g gs ->
            List.Extra.findIndex predicate gs
                |> Maybe.map (\i -> List.Extra.setAt i g gs)
                |> Maybe.withDefault gs
        )


type alias PipelineIndex =
    Int


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
        Dragging name _ ->
            Just name

        NotDragging ->
            Nothing


setTeamName : Concourse.TeamName -> DragState -> DragState
setTeamName name dragState =
    case dragState of
        Dragging _ dragIdx ->
            Dragging name dragIdx

        NotDragging ->
            NotDragging


dragIndex : DragState -> Maybe PipelineIndex
dragIndex dragState =
    case dragState of
        Dragging _ dragIdx ->
            Just dragIdx

        NotDragging ->
            Nothing


setDragIndex : PipelineIndex -> DragState -> DragState
setDragIndex dragIdx dragState =
    case dragState of
        Dragging name _ ->
            Dragging name dragIdx

        NotDragging ->
            NotDragging


dropIndex : DropState -> Maybe PipelineIndex
dropIndex dropState =
    case dropState of
        Dropping dropIdx ->
            Just dropIdx

        NotDropping ->
            Nothing


setDropIndex : PipelineIndex -> DropState -> DropState
setDropIndex dropIdx dropState =
    case dropState of
        Dropping _ ->
            Dropping dropIdx

        NotDropping ->
            NotDropping


allPipelines : Concourse.APIData -> List Pipeline
allPipelines data =
    data.pipelines
        |> List.map
            (\p ->
                let
                    jobs =
                        data.jobs
                            |> List.filter
                                (\j ->
                                    (j.teamName == p.teamName)
                                        && (j.pipelineName == p.name)
                                )
                in
                { id = p.id
                , name = p.name
                , teamName = p.teamName
                , public = p.public
                , jobs = jobs
                , resourceError =
                    data.resources
                        |> List.any
                            (\r ->
                                (r.teamName == p.teamName)
                                    && (r.pipelineName == p.name)
                                    && r.failingToCheck
                            )
                , status = pipelineStatus p jobs
                , isToggleLoading = False
                }
            )


pipelineStatus : Concourse.Pipeline -> List Concourse.Job -> PipelineStatus.PipelineStatus
pipelineStatus pipeline jobs =
    if pipeline.paused then
        PipelineStatus.PipelineStatusPaused

    else
        let
            isRunning =
                List.any (\job -> job.nextBuild /= Nothing) jobs

            mostImportantJobStatus =
                jobs
                    |> List.map jobStatus
                    |> List.sortWith Concourse.BuildStatus.ordering
                    |> List.head

            firstNonSuccess =
                jobs
                    |> List.filter (jobStatus >> (/=) Concourse.BuildStatusSucceeded)
                    |> List.filterMap transition
                    |> List.sortBy Time.posixToMillis
                    |> List.head

            lastTransition =
                jobs
                    |> List.filterMap transition
                    |> List.sortBy Time.posixToMillis
                    |> List.reverse
                    |> List.head

            transitionTime =
                case firstNonSuccess of
                    Just t ->
                        Just t

                    Nothing ->
                        lastTransition
        in
        case ( mostImportantJobStatus, transitionTime ) of
            ( _, Nothing ) ->
                PipelineStatus.PipelineStatusPending isRunning

            ( Nothing, _ ) ->
                PipelineStatus.PipelineStatusPending isRunning

            ( Just Concourse.BuildStatusPending, _ ) ->
                PipelineStatus.PipelineStatusPending isRunning

            ( Just Concourse.BuildStatusStarted, _ ) ->
                PipelineStatus.PipelineStatusPending isRunning

            ( Just Concourse.BuildStatusSucceeded, Just since ) ->
                if isRunning then
                    PipelineStatus.PipelineStatusSucceeded PipelineStatus.Running

                else
                    PipelineStatus.PipelineStatusSucceeded (PipelineStatus.Since since)

            ( Just Concourse.BuildStatusFailed, Just since ) ->
                if isRunning then
                    PipelineStatus.PipelineStatusFailed PipelineStatus.Running

                else
                    PipelineStatus.PipelineStatusFailed (PipelineStatus.Since since)

            ( Just Concourse.BuildStatusErrored, Just since ) ->
                if isRunning then
                    PipelineStatus.PipelineStatusErrored PipelineStatus.Running

                else
                    PipelineStatus.PipelineStatusErrored (PipelineStatus.Since since)

            ( Just Concourse.BuildStatusAborted, Just since ) ->
                if isRunning then
                    PipelineStatus.PipelineStatusAborted PipelineStatus.Running

                else
                    PipelineStatus.PipelineStatusAborted (PipelineStatus.Since since)


jobStatus : Concourse.Job -> Concourse.BuildStatus
jobStatus job =
    case job.finishedBuild of
        Just build ->
            build.status

        Nothing ->
            Concourse.BuildStatusPending


transition : Concourse.Job -> Maybe Time.Posix
transition =
    .transitionBuild >> Maybe.andThen (.duration >> .finishedAt)


shiftPipelines : Int -> Int -> Group -> Group
shiftPipelines dragIdx dropIdx g =
    if dragIdx == dropIdx then
        g

    else
        let
            pipelines =
                case
                    List.head <|
                        List.drop dragIdx <|
                            g.pipelines
                of
                    Nothing ->
                        g.pipelines

                    Just pipeline ->
                        shiftPipelineTo pipeline dropIdx g.pipelines
        in
        { g | pipelines = pipelines }


shiftPipelineTo : Pipeline -> Int -> List Pipeline -> List Pipeline
shiftPipelineTo pipeline position pipelines =
    case pipelines of
        [] ->
            if position < 0 then
                []

            else
                [ pipeline ]

        p :: ps ->
            if p.teamName /= pipeline.teamName then
                p :: shiftPipelineTo pipeline position ps

            else if p == pipeline then
                shiftPipelineTo pipeline (position - 1) ps

            else if position == 0 then
                pipeline :: p :: shiftPipelineTo pipeline (position - 1) ps

            else
                p :: shiftPipelineTo pipeline (position - 1) ps


allTeamNames : Concourse.APIData -> List String
allTeamNames apiData =
    Set.union
        (Set.fromList (List.map .teamName apiData.pipelines))
        (Set.fromList (List.map .name apiData.teams))
        |> Set.toList


groups : Concourse.APIData -> List Group
groups apiData =
    let
        teamNames =
            allTeamNames apiData
    in
    teamNames
        |> List.map (group (allPipelines apiData) apiData.user)


group : List Pipeline -> Maybe Concourse.User -> String -> Group
group pipelines user name =
    { pipelines = List.filter (.teamName >> (==) name) pipelines
    , teamName = name
    , tag = user |> Maybe.andThen (\u -> Tag.tag u name)
    }


view :
    { dragState : DragState
    , dropState : DropState
    , now : Time.Posix
    , hovered : Maybe DomID
    , pipelineRunningKeyframes : String
    , userState : UserState
    }
    -> Group
    -> Html Message
view { dragState, dropState, now, hovered, pipelineRunningKeyframes, userState } g =
    let
        pipelines =
            if List.isEmpty g.pipelines then
                [ Pipeline.pipelineNotSetView ]

            else
                List.append
                    (List.indexedMap
                        (\i pipeline ->
                            Html.div [ class "pipeline-wrapper" ]
                                [ pipelineDropAreaView dragState dropState g.teamName i
                                , Html.div
                                    [ classList
                                        [ ( "card", True )
                                        , ( "dragging"
                                          , dragState == Dragging pipeline.teamName i
                                          )
                                        ]
                                    , attribute "data-pipeline-name" pipeline.name
                                    , attribute
                                        "ondragstart"
                                        "event.dataTransfer.setData('text/plain', '');"
                                    , draggable "true"
                                    , on "dragstart"
                                        (Json.Decode.succeed (DragStart pipeline.teamName i))
                                    , on "dragend" (Json.Decode.succeed DragEnd)
                                    ]
                                    [ Pipeline.pipelineView
                                        { now = now
                                        , pipeline = pipeline
                                        , hovered = hovered
                                        , pipelineRunningKeyframes = pipelineRunningKeyframes
                                        , userState = userState
                                        }
                                    ]
                                ]
                        )
                        g.pipelines
                    )
                    [ pipelineDropAreaView dragState dropState g.teamName (List.length g.pipelines) ]
    in
    Html.div
        [ id g.teamName
        , class "dashboard-team-group"
        , attribute "data-team-name" g.teamName
        ]
        [ Html.div
            [ style "display" "flex"
            , style "align-items" "center"
            , class <| .sectionHeaderClass Effects.stickyHeaderConfig
            ]
            (Html.div
                [ class "dashboard-team-name" ]
                [ Html.text g.teamName ]
                :: (Maybe.Extra.toList <|
                        Maybe.map (Tag.view False) g.tag
                   )
            )
        , Html.div
            [ class <| .sectionBodyClass Effects.stickyHeaderConfig ]
            pipelines
        ]


hdView : String -> Group -> List (Html Message)
hdView pipelineRunningKeyframes g =
    let
        header =
            Html.div
                [ class "dashboard-team-name" ]
                [ Html.text g.teamName ]
                :: (Maybe.Extra.toList <| Maybe.map (Tag.view True) g.tag)

        teamPipelines =
            if List.isEmpty g.pipelines then
                [ pipelineNotSetView ]

            else
                g.pipelines
                    |> List.map
                        (\p ->
                            Pipeline.hdPipelineView
                                { pipeline = p
                                , pipelineRunningKeyframes = pipelineRunningKeyframes
                                }
                        )
    in
    case teamPipelines of
        [] ->
            header

        p :: ps ->
            -- Wrap the team name and the first pipeline together so
            -- the team name is not the last element in a column
            Html.div
                (class "dashboard-team-name-wrapper" :: Styles.teamNameHd)
                (header ++ [ p ])
                :: ps


pipelineNotSetView : Html Message
pipelineNotSetView =
    Html.div
        [ class "card" ]
        [ Html.div
            Styles.noPipelineCardHd
            [ Html.div
                Styles.noPipelineCardTextHd
                [ Html.text "no pipelines set" ]
            ]
        ]


pipelineDropAreaView : DragState -> DropState -> String -> Int -> Html Message
pipelineDropAreaView dragState dropState name index =
    let
        ( active, over ) =
            case ( dragState, dropState ) of
                ( Dragging team dragIdx, NotDropping ) ->
                    ( team == name, index == dragIdx )

                ( Dragging team _, Dropping dropIdx ) ->
                    ( team == name, index == dropIdx )

                _ ->
                    ( False, False )
    in
    Html.div
        [ classList [ ( "drop-area", True ), ( "active", active ), ( "over", over ), ( "animation", dropState /= NotDropping ) ]
        , on "dragenter" (Json.Decode.succeed (DragOver name index))
        ]
        [ Html.text "" ]
