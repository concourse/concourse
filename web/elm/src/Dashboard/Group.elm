module Dashboard.Group exposing
    ( PipelineIndex
    , hdView
    , ordering
    , pipelineNotSetView
    , view
    )

import Concourse
import Dashboard.Group.Models exposing (Group, Pipeline)
import Dashboard.Group.Tag as Tag
import Dashboard.Models exposing (DragState(..), DropState(..))
import Dashboard.Pipeline as Pipeline
import Dashboard.PipelineGrid as PipelineGrid
import Dashboard.PipelineGrid.Constants as PipelineGridConstants
import Dashboard.Styles as Styles
import Dict exposing (Dict)
import HoverState
import Html exposing (Html)
import Html.Attributes exposing (attribute, class, classList, draggable, id, style)
import Html.Events exposing (on, preventDefaultOn, stopPropagationOn)
import Html.Keyed
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
        , pipelineLayers : Dict ( String, String ) (List (List Concourse.Job))
        , query : String
        , pipelineCards : List PipelineGrid.PipelineCard
        , dropAreas : List PipelineGrid.DropArea
        , groupCardsHeight : Float
        , pipelineJobs : Dict ( String, String ) (List Concourse.Job)
        , isCached : Bool
        }
    -> Group
    -> Html Message
view session params g =
    let
        pipelineCardViews =
            if List.isEmpty params.pipelineCards then
                [ ( "not-set", Pipeline.pipelineNotSetView ) ]

            else
                params.pipelineCards
                    |> List.map
                        (\{ bounds, pipeline, index } ->
                            pipelineCardView session
                                params
                                { bounds = bounds, pipeline = pipeline, index = index }
                                g.teamName
                                |> (\html -> ( String.fromInt pipeline.id, html ))
                        )

        dropAreaViews =
            params.dropAreas
                |> List.map
                    (\{ bounds, index } ->
                        pipelineDropAreaView params.dragState g.teamName bounds index
                    )
    in
    Html.div
        [ id <| Effects.toHtmlID <| DashboardGroup g.teamName
        , class "dashboard-team-group"
        , attribute "data-team-name" g.teamName
        ]
        [ Html.div
            [ style "display" "flex"
            , style "align-items" "center"
            , style "margin-bottom" (String.fromInt PipelineGridConstants.padding ++ "px")
            , class <| .sectionHeaderClass Effects.stickyHeaderConfig
            ]
            (Html.div
                [ class "dashboard-team-name" ]
                [ Html.text g.teamName ]
                :: (Maybe.Extra.toList <|
                        Maybe.map (Tag.view False) (tag session g)
                   )
                ++ (if params.dropState == DroppingWhileApiRequestInFlight g.teamName then
                        [ Spinner.spinner { sizePx = 20, margin = "0 0 0 10px" } ]

                    else
                        []
                   )
            )
        , Html.Keyed.node "div"
            [ class <| .sectionBodyClass Effects.stickyHeaderConfig
            , style "position" "relative"
            , style "height" <| String.fromFloat params.groupCardsHeight ++ "px"
            ]
            (pipelineCardViews
                ++ [ ( "drop-areas", Html.div [ style "position" "absolute" ] dropAreaViews ) ]
            )
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
    , pipelineJobs : Dict ( String, String ) (List Concourse.Job)
    , isCached : Bool
    }
    -> { a | userState : UserState }
    -> Group
    -> List (Html Message)
hdView { pipelineRunningKeyframes, pipelinesWithResourceErrors, pipelineJobs, isCached } session g =
    let
        orderedPipelines =
            g.pipelines

        header =
            Html.div
                [ class "dashboard-team-name" ]
                [ Html.text g.teamName ]
                :: (Maybe.Extra.toList <| Maybe.map (Tag.view True) (tag session g))

        teamPipelines =
            if List.isEmpty orderedPipelines then
                [ pipelineNotSetView ]

            else
                orderedPipelines
                    |> List.map
                        (\p ->
                            Pipeline.hdPipelineView
                                { pipeline = p
                                , pipelineRunningKeyframes = pipelineRunningKeyframes
                                , resourceError =
                                    pipelinesWithResourceErrors
                                        |> Dict.get ( p.teamName, p.name )
                                        |> Maybe.withDefault False
                                , existingJobs =
                                    pipelineJobs
                                        |> Dict.get ( p.teamName, p.name )
                                        |> Maybe.withDefault []
                                , isCached = isCached
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


pipelineCardView :
    { a | userState : UserState }
    ->
        { b
            | dragState : DragState
            , dropState : DropState
            , now : Maybe Time.Posix
            , hovered : HoverState.HoverState
            , pipelineRunningKeyframes : String
            , pipelinesWithResourceErrors : Dict ( String, String ) Bool
            , pipelineLayers : Dict ( String, String ) (List (List Concourse.Job))
            , query : String
            , pipelineJobs : Dict ( String, String ) (List Concourse.Job)
            , isCached : Bool
        }
    ->
        { bounds : PipelineGrid.Bounds
        , pipeline : Pipeline
        , index : Int
        }
    -> String
    -> Html Message
pipelineCardView session params { bounds, pipeline, index } teamName =
    Html.div
        ([ class "pipeline-wrapper"
         , style "position" "absolute"
         , style "transform"
            ("translate("
                ++ String.fromFloat bounds.x
                ++ "px,"
                ++ String.fromFloat bounds.y
                ++ "px)"
            )
         , style
            "width"
            (String.fromFloat bounds.width
                ++ "px"
            )
         , style "height"
            (String.fromFloat bounds.height
                ++ "px"
            )
         ]
            ++ (if params.dragState /= NotDragging then
                    [ style "transition" "transform 0.2s ease-in-out" ]

                else
                    []
               )
            ++ (case HoverState.hoveredElement params.hovered of
                    Just (JobPreview jobID) ->
                        if
                            (jobID.teamName == pipeline.teamName)
                                && (jobID.pipelineName == pipeline.name)
                        then
                            [ style "z-index" "1" ]

                        else
                            []

                    _ ->
                        []
               )
        )
        [ Html.div
            ([ class "card"
             , style "width" "100%"
             , attribute "data-pipeline-name" pipeline.name
             ]
                ++ (if not params.isCached && String.isEmpty params.query then
                        [ attribute
                            "ondragstart"
                            "event.dataTransfer.setData('text/plain', '');"
                        , draggable "true"
                        , on "dragstart"
                            (Json.Decode.succeed (DragStart pipeline.teamName index))
                        , on "dragend" (Json.Decode.succeed DragEnd)
                        ]

                    else
                        []
                   )
                ++ (if params.dragState == Dragging pipeline.teamName index then
                        [ style "width" "0"
                        , style "margin" "0 12.5px"
                        , style "overflow" "hidden"
                        ]

                    else
                        []
                   )
                ++ (if params.dropState == DroppingWhileApiRequestInFlight teamName then
                        [ style "opacity" "0.45", style "pointer-events" "none" ]

                    else
                        [ style "opacity" "1" ]
                   )
            )
            [ Pipeline.pipelineView
                { now = params.now
                , pipeline = pipeline
                , resourceError =
                    params.pipelinesWithResourceErrors
                        |> Dict.get ( pipeline.teamName, pipeline.name )
                        |> Maybe.withDefault False
                , existingJobs =
                    params.pipelineJobs
                        |> Dict.get ( pipeline.teamName, pipeline.name )
                        |> Maybe.withDefault []
                , layers =
                    params.pipelineLayers
                        |> Dict.get ( pipeline.teamName, pipeline.name )
                        |> Maybe.withDefault []
                , hovered = params.hovered
                , pipelineRunningKeyframes = params.pipelineRunningKeyframes
                , userState = session.userState
                , query = params.query
                , isCached = params.isCached
                }
            ]
        ]


pipelineDropAreaView : DragState -> String -> PipelineGrid.Bounds -> Int -> Html Message
pipelineDropAreaView dragState name { x, y, width, height } index =
    let
        active =
            case dragState of
                Dragging team _ ->
                    team == name

                _ ->
                    False
    in
    Html.div
        [ classList
            [ ( "drop-area", True )
            , ( "active", active )
            ]
        , style "position" "absolute"
        , style "transform" <|
            "translate("
                ++ String.fromFloat x
                ++ "px,"
                ++ String.fromFloat y
                ++ "px)"
        , style "width" <| String.fromFloat width ++ "px"
        , style "height" <| String.fromFloat height ++ "px"
        , on "dragenter" (Json.Decode.succeed (DragOver name index))

        -- preventDefault is required so that the card will not appear to
        -- "float" or "snap" back to its original position when dropped.
        , preventDefaultOn "dragover" (Json.Decode.succeed ( DragOver name index, True ))
        , stopPropagationOn "drop" (Json.Decode.succeed ( DragEnd, True ))
        ]
        []
