module Dashboard.Group exposing
    ( PipelineIndex
    , allTeamNames
    , dragIndex
    , dragIndexOptional
    , dropIndex
    , dropIndexOptional
    , findGroupOptional
    , group
    , groups
    , hdView
    , ordering
    , pipelineDropAreaView
    , pipelineNotSetView
    , setDragIndex
    , setDropIndex
    , setTeamName
    , shiftPipelineTo
    , shiftPipelines
    , teamName
    , teamNameOptional
    , view
    )

import Concourse
import Dashboard.Group.Models exposing (Group, Pipeline)
import Dashboard.Group.Tag as Tag
import Dashboard.Models exposing (DragState(..), DropState(..))
import Dashboard.Pipeline as Pipeline
import Dashboard.Styles as Styles
import Dict exposing (Dict)
import HoverState
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
import Time
import UserState exposing (UserState(..))


ordering : { a | userState : UserState } -> Ordering Group
ordering session =
    Ordering.byFieldWith Tag.ordering (tag session)
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
    List.map .name apiData.teams


groups : Concourse.APIData -> List Group
groups apiData =
    let
        teamNames =
            allTeamNames apiData
    in
    teamNames
        |> List.map group


group : String -> Group
group name =
    { pipelines = []
    , teamName = name
    }


view :
    { a | userState : UserState }
    ->
        { dragState : DragState
        , dropState : DropState
        , now : Time.Posix
        , hovered : HoverState.HoverState
        , pipelineRunningKeyframes : String
        , pipelinesWithResourceErrors : Dict ( String, String ) Bool
        , existingJobs : List Concourse.Job
        }
    -> Group
    -> Html Message
view session { dragState, dropState, now, hovered, pipelineRunningKeyframes, pipelinesWithResourceErrors, existingJobs } g =
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
                                        , resourceError =
                                            pipelinesWithResourceErrors
                                                |> Dict.get ( pipeline.teamName, pipeline.name )
                                                |> Maybe.withDefault False
                                        , existingJobs =
                                            existingJobs
                                                |> List.filter
                                                    (\j ->
                                                        j.teamName == pipeline.teamName && j.pipelineName == pipeline.name
                                                    )
                                        , hovered = hovered
                                        , pipelineRunningKeyframes = pipelineRunningKeyframes
                                        , userState = session.userState
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
                        Maybe.map (Tag.view False) (tag session g)
                   )
            )
        , Html.div
            [ class <| .sectionBodyClass Effects.stickyHeaderConfig ]
            pipelines
        ]


tag : { a | userState : UserState } -> Group -> Maybe Tag.Tag
tag { userState } g =
    case userState of
        UserStateLoggedIn user ->
            Tag.tag user g.teamName

        _ ->
            Nothing


hdView :
    { pipelineRunningKeyframes : String, pipelinesWithResourceErrors : Dict ( String, String ) Bool, existingJobs : List Concourse.Job }
    -> { a | userState : UserState }
    -> Group
    -> List (Html Message)
hdView { pipelineRunningKeyframes, pipelinesWithResourceErrors, existingJobs } session g =
    let
        header =
            Html.div
                [ class "dashboard-team-name" ]
                [ Html.text g.teamName ]
                :: (Maybe.Extra.toList <| Maybe.map (Tag.view True) (tag session g))

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
                                , resourceError =
                                    pipelinesWithResourceErrors
                                        |> Dict.get ( p.teamName, p.name )
                                        |> Maybe.withDefault False
                                , existingJobs = existingJobs
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
