module Dashboard.Group exposing
    ( PipelineIndex
    , hdView
    , ordering
    , pipelineDropAreaView
    , pipelineNotSetView
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
import Html.Events exposing (on, preventDefaultOn, stopPropagationOn)
import Json.Decode
import Maybe.Extra
import Message.Effects as Effects
import Message.Message exposing (DomID(..), Message(..))
import Ordering exposing (Ordering)
import Time
import UserState exposing (UserState(..))
import Views.Spinner as Spinner


ordering : { a | userState : UserState } -> Ordering Group
ordering session =
    Ordering.byFieldWith Tag.ordering (tag session)
        |> Ordering.breakTiesWith (Ordering.byField .teamName)


type alias PipelineIndex =
    Int


view :
    { a | userState : UserState }
    ->
        { dragState : DragState
        , dropState : DropState
        , now : Maybe Time.Posix
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
            g.pipelines

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
                                        ++ (if dropState == DroppingWhileApiRequestInFlight g.teamName then
                                                [ style "opacity" "0.45", style "pointer-events" "none" ]

                                            else
                                                [ style "opacity" "1" ]
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
                ++ (if dropState == DroppingWhileApiRequestInFlight g.teamName then
                        [ Spinner.spinner { sizePx = 20, margin = "0 0 0 10px" } ]

                    else
                        []
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
            g.pipelines

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
        , stopPropagationOn "drop" (Json.Decode.succeed ( DragEnd, True ))
        , style "padding" <|
            "0 "
                ++ (if active && over then
                        "198.5px"

                    else
                        "50px"
                   )
        ]
        []
