module Dashboard.Group exposing
    ( PipelineIndex
    , hdView
    , ordering
    , pipelineDropAreaView
    , pipelineNotSetView
    , view
    )

import Concourse
import Dashboard.Drag exposing (drag)
import Dashboard.Group.Models exposing (Group, Pipeline)
import Dashboard.Group.Tag as Tag
import Dashboard.Models exposing (DragState(..), DropState(..))
import Dashboard.Pipeline as Pipeline
import Dashboard.PipelineGridLayout as PipelineGridLayout exposing (cardHeight, cardWidth, padding)
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


type alias Bounds =
    { x : Float
    , y : Float
    , width : Float
    , height : Float
    }


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
        , pipelineLayers : Dict ( String, String ) (List (List Concourse.Job))
        , viewportWidth : Float
        , viewportHeight : Float
        , scrollTop : Float
        , query : String
        }
    -> Group
    -> ( Html Message, Float )
view session params g =
    let
        ( dragTeam, fromIndex, toIndex ) =
            case ( params.dragState, params.dropState ) of
                ( Dragging team fromIdx, NotDropping ) ->
                    ( team, fromIdx, fromIdx + 1 )

                ( Dragging team fromIdx, Dropping toIdx ) ->
                    ( team, fromIdx, toIdx )

                _ ->
                    ( "", -1, -1 )

        orderedPipelines =
            if (g.teamName == dragTeam) && (fromIndex >= 0) then
                drag fromIndex toIndex g.pipelines
                    |> List.indexedMap (\i p -> { p | ordering = i })

            else
                g.pipelines

        headerHeight =
            60

        numColumns =
            max 1 (floor (params.viewportWidth / (cardWidth + padding)))

        numRowsVisible =
            ceiling (params.viewportHeight / (cardHeight + padding)) + 1

        numRowsOffset =
            floor (params.scrollTop / (cardHeight + padding))

        isVisible { row, height } =
            (numRowsOffset < row + height)
                && (row <= numRowsOffset + numRowsVisible)

        previewSizes =
            orderedPipelines
                |> List.map
                    (\pipeline ->
                        Dict.get ( pipeline.name, pipeline.teamName ) params.pipelineLayers
                            |> Maybe.withDefault []
                    )
                |> List.map
                    (\layers ->
                        ( List.length layers
                        , layers
                            |> List.map List.length
                            |> List.maximum
                            |> Maybe.withDefault 0
                        )
                    )

        cards =
            previewSizes
                |> List.map PipelineGridLayout.cardSize
                |> PipelineGridLayout.layout numColumns

        numRows =
            cards
                |> List.map (\c -> c.row + c.height - 1)
                |> List.maximum
                |> Maybe.withDefault 1

        totalCardsHeight =
            numRows
                * cardHeight
                + padding
                * (numRows - 1)

        cardLookup =
            cards
                |> List.map2 Tuple.pair orderedPipelines
                |> List.map (\( pipeline, card ) -> ( pipeline.id, card ))
                |> Dict.fromList

        prevAndCurrentCards =
            cards
                |> List.map2 Tuple.pair (Nothing :: (cards |> List.map Just))

        dropAreas =
            (prevAndCurrentCards
                |> List.indexedMap Tuple.pair
                |> List.filter (\( _, ( _, card ) ) -> isVisible card)
                |> List.map
                    (\( i, ( prevCard, card ) ) ->
                        let
                            cardBounds =
                                { x = (toFloat card.column - 1) * (cardWidth + padding)
                                , y = (toFloat card.row - 1) * (cardHeight + padding)
                                , width = cardWidth * toFloat card.width + padding * toFloat card.width
                                , height = cardHeight * toFloat card.height + padding * (toFloat card.height - 1)
                                }

                            boundsToRightOf otherCard =
                                { x = toFloat (otherCard.column - 1 + otherCard.width) * (cardWidth + padding)
                                , y = (toFloat otherCard.row - 1) * (cardHeight + padding)
                                , width = cardWidth + padding
                                , height = cardHeight
                                }

                            bounds =
                                case prevCard of
                                    Just otherCard ->
                                        if
                                            (otherCard.row < card.row)
                                                && (otherCard.column + otherCard.width <= numColumns)
                                        then
                                            boundsToRightOf otherCard

                                        else
                                            cardBounds

                                    Nothing ->
                                        cardBounds
                        in
                        pipelineDropAreaView params.dragState
                            g.teamName
                            bounds
                            (i + 1)
                    )
            )
                ++ (case List.head (List.reverse cards) of
                        Just lastCard ->
                            if not (isVisible lastCard) then
                                []

                            else
                                [ pipelineDropAreaView params.dragState
                                    g.teamName
                                    { x = toFloat lastCard.column * (cardWidth + padding)
                                    , y = (toFloat lastCard.row - 1) * (cardHeight + padding)
                                    , width = cardWidth + padding
                                    , height = cardHeight
                                    }
                                    (List.length cards + 1)
                                ]

                        Nothing ->
                            []
                   )

        pipelineCards =
            if List.isEmpty orderedPipelines then
                [ ( "not-set", Pipeline.pipelineNotSetView ) ]

            else
                g.pipelines
                    |> List.map
                        (\p ->
                            cardLookup
                                |> Dict.get p.id
                                |> Maybe.withDefault { row = 0, column = 0, width = 0, height = 0 }
                                |> Tuple.pair p
                        )
                    |> List.filter (\( _, card ) -> isVisible card)
                    |> List.map
                        (\( pipeline, card ) ->
                            params.pipelineLayers
                                |> Dict.get ( pipeline.name, pipeline.teamName )
                                |> Maybe.withDefault []
                                |> (\layers -> ( pipeline, card, layers ))
                        )
                    |> List.map
                        (\( pipeline, card, layers ) ->
                            Html.div
                                ([ class "pipeline-wrapper"
                                 , style "position" "absolute"
                                 , style "transform"
                                    ("translate("
                                        ++ String.fromInt ((card.column - 1) * (cardWidth + padding) + padding)
                                        ++ "px,"
                                        ++ String.fromInt ((card.row - 1) * (cardHeight + padding))
                                        ++ "px)"
                                    )
                                 , style
                                    "width"
                                    (String.fromInt
                                        (cardWidth
                                            * card.width
                                            + padding
                                            * (card.width - 1)
                                        )
                                        ++ "px"
                                    )
                                 , style "height"
                                    (String.fromInt
                                        (cardHeight
                                            * card.height
                                            + padding
                                            * (card.height - 1)
                                        )
                                        ++ "px"
                                    )
                                 ]
                                    ++ (if dragTeam == g.teamName then
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
                                     , id <| Effects.toHtmlID <| PipelineCard pipeline.id
                                     , attribute "data-pipeline-name" pipeline.name
                                     ]
                                        ++ (if String.isEmpty params.query then
                                                [ attribute
                                                    "ondragstart"
                                                    "event.dataTransfer.setData('text/plain', '');"
                                                , draggable "true"
                                                , on "dragstart"
                                                    (Json.Decode.succeed (DragStart pipeline.teamName pipeline.ordering))
                                                , on "dragend" (Json.Decode.succeed DragEnd)
                                                ]

                                            else
                                                []
                                           )
                                        ++ (if params.dragState == Dragging pipeline.teamName pipeline.ordering then
                                                [ style "width" "0"
                                                , style "margin" "0 12.5px"
                                                , style "overflow" "hidden"
                                                ]

                                            else
                                                []
                                           )
                                        ++ (if params.dropState == DroppingWhileApiRequestInFlight g.teamName then
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
                                            params.existingJobs
                                                |> List.filter
                                                    (\j ->
                                                        j.teamName == pipeline.teamName && j.pipelineName == pipeline.name
                                                    )
                                        , layers = layers
                                        , hovered = params.hovered
                                        , pipelineRunningKeyframes = params.pipelineRunningKeyframes
                                        , userState = session.userState
                                        , query = params.query
                                        }
                                    ]
                                ]
                                |> Tuple.pair (String.fromInt pipeline.id)
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
            , style "height" <| String.fromInt totalCardsHeight ++ "px"
            ]
            (pipelineCards ++ [ ( "drop-areas", Html.div [ style "position" "absolute" ] dropAreas ) ])
        ]
        |> (\html ->
                ( html
                , toFloat <| totalCardsHeight + headerHeight
                )
           )


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
    }
    -> { a | userState : UserState }
    -> Group
    -> List (Html Message)
hdView { pipelineRunningKeyframes, pipelinesWithResourceErrors, existingJobs } session g =
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


pipelineDropAreaView : DragState -> String -> Bounds -> Int -> Html Message
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
