module Dashboard.Group exposing
    ( PipelineIndex
    , dragIndex
    , dropIndex
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
    , view
    )

import Concourse
import Dashboard.Group.Models exposing (Group, Pipeline)
import Dashboard.Group.Tag as Tag
import Dashboard.Models exposing (DashboardError, DragState(..), DropState(..))
import Dashboard.Pipeline as Pipeline
import Dashboard.Styles as Styles
import Dict exposing (Dict)
import HoverState
import Html exposing (Html)
import Html.Attributes exposing (attribute, class, classList, draggable, id, style)
import Html.Events exposing (on, preventDefaultOn)
import Json.Decode
import Maybe.Extra
import Message.Effects as Effects
import Message.Message exposing (DomID(..), Message(..))
import Ordering exposing (Ordering)
import RemoteData exposing (RemoteData)
import Time
import UserState exposing (UserState(..))


ordering : { a | userState : UserState } -> Ordering Group
ordering session =
    Ordering.byFieldWith Tag.ordering (tag session)
        |> Ordering.breakTiesWith (Ordering.byField .teamName)


type alias PipelineIndex =
    Int


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


view :
    { a | userState : UserState }
    ->
        { dragState : DragState
        , dropState : DropState
        , now : RemoteData DashboardError Time.Posix
        , hovered : HoverState.HoverState
        , pipelineRunningKeyframes : String
        , pipelinesWithResourceErrors : Dict ( String, String ) Bool
        , existingJobs : List Concourse.Job
        , pipelines : List Pipeline
        }
    -> Group
    -> Html Message
view session { dragState, dropState, now, hovered, pipelineRunningKeyframes, pipelinesWithResourceErrors, existingJobs, pipelines } g =
    let
        pipelinesForGroup =
            pipelines |> List.filter (.teamName >> (==) g.teamName)

        pipelineCards =
            if List.isEmpty pipelinesForGroup then
                [ Pipeline.pipelineNotSetView ]

            else
                List.append
                    (List.map
                        (\pipeline ->
                            Html.div [ class "pipeline-wrapper" ]
                                [ pipelineDropAreaView dragState dropState g.teamName pipeline.ordering
                                , Html.div
                                    ([ class "card"
                                     , attribute "data-pipeline-name" pipeline.name
                                     , attribute
                                        "ondragstart"
                                        "event.dataTransfer.setData('text/plain', '');"
                                     , draggable "true"
                                     , on "dragstart"
                                        (Json.Decode.succeed (DragStart pipeline.teamName pipeline.ordering))
                                     , on "dragend" (Json.Decode.succeed DragEnd)
                                     ]
                                        ++ (if dragState == Dragging pipeline.teamName pipeline.ordering then
                                                [ style "width" "0"
                                                , style "margin" "0 12.5px"
                                                , style "overflow" "hidden"
                                                ]

                                            else
                                                []
                                           )
                                    )
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
                        pipelinesForGroup
                    )
                    [ pipelineDropAreaView dragState
                        dropState
                        g.teamName
                        (pipelinesForGroup
                            |> List.map (.ordering >> (+) 1)
                            |> List.maximum
                            |> Maybe.withDefault 0
                        )
                    ]
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
            pipelineCards
        ]


tag : { a | userState : UserState } -> Group -> Maybe Tag.Tag
tag { userState } g =
    case userState of
        UserStateLoggedIn user ->
            Tag.tag user g.teamName

        _ ->
            Nothing


hdView :
    { pipelineRunningKeyframes : String
    , pipelinesWithResourceErrors : Dict ( String, String ) Bool
    , existingJobs : List Concourse.Job
    , pipelines : List Pipeline
    }
    -> { a | userState : UserState }
    -> Group
    -> List (Html Message)
hdView { pipelineRunningKeyframes, pipelinesWithResourceErrors, existingJobs, pipelines } session g =
    let
        pipelinesForGroup =
            pipelines |> List.filter (.teamName >> (==) g.teamName)

        header =
            Html.div
                [ class "dashboard-team-name" ]
                [ Html.text g.teamName ]
                :: (Maybe.Extra.toList <| Maybe.map (Tag.view True) (tag session g))

        teamPipelines =
            if List.isEmpty pipelinesForGroup then
                [ pipelineNotSetView ]

            else
                pipelinesForGroup
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
        [ classList
            [ ( "drop-area", True )
            , ( "active", active )
            , ( "animation", dropState /= NotDropping )
            ]
        , on "dragenter" (Json.Decode.succeed (DragOver name index))

        -- preventDefault is required so that the card will not appear to
        -- "float" or "snap" back to its original position when dropped.
        , preventDefaultOn "dragover" (Json.Decode.succeed ( DragOver name index, True ))
        , style "padding" <|
            "0 "
                ++ (if active && over then
                        "198.5px"

                    else
                        "50px"
                   )
        ]
        []
