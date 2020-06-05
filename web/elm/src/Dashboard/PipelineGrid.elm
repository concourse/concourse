module Dashboard.PipelineGrid exposing
    ( Bounds
    , DropArea
    , PipelineCard
    , computeLayout
    )

import Concourse
import Dashboard.Drag exposing (dragPipeline)
import Dashboard.Group.Models exposing (Group, Pipeline)
import Dashboard.Models exposing (DragState(..), DropState(..))
import Dashboard.PipelineGrid.Constants exposing (cardHeight, cardWidth, padding)
import Dashboard.PipelineGrid.Layout as Layout
import Dict exposing (Dict)
import Message.Message exposing (DomID(..), DropTarget(..), Message(..))
import UserState exposing (UserState(..))


type alias Bounds =
    { x : Float
    , y : Float
    , width : Float
    , height : Float
    }


type alias PipelineCard =
    { bounds : Bounds
    , pipeline : Pipeline
    }


type alias DropArea =
    { bounds : Bounds
    , target : DropTarget
    }


computeLayout :
    { dragState : DragState
    , dropState : DropState
    , pipelineLayers : Dict ( String, String ) (List (List Concourse.JobIdentifier))
    , viewportWidth : Float
    , viewportHeight : Float
    , scrollTop : Float
    }
    -> Group
    ->
        { pipelineCards : List PipelineCard
        , dropAreas : List DropArea
        , height : Float
        }
computeLayout params g =
    let
        orderedPipelines =
            case ( params.dragState, params.dropState ) of
                ( Dragging team pipeline, Dropping target ) ->
                    if g.teamName == team then
                        dragPipeline pipeline target g.pipelines

                    else
                        g.pipelines

                _ ->
                    g.pipelines

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
                        Dict.get ( pipeline.teamName, pipeline.name ) params.pipelineLayers
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
                |> List.map Layout.cardSize
                |> Layout.layout numColumns

        numRows =
            cards
                |> List.map (\c -> c.row + c.height - 1)
                |> List.maximum
                |> Maybe.withDefault 1

        totalCardsHeight =
            toFloat numRows
                * cardHeight
                + padding
                * toFloat numRows

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
                |> List.map2 Tuple.pair g.pipelines
                |> List.filter (\( _, ( _, card ) ) -> isVisible card)
                |> List.map
                    (\( pipeline, ( prevCard, card ) ) ->
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
                        { bounds = bounds, target = Before pipeline.name }
                    )
            )
                ++ (case List.head (List.reverse (List.map2 Tuple.pair cards g.pipelines)) of
                        Just ( lastCard, lastPipeline ) ->
                            if not (isVisible lastCard) then
                                []

                            else
                                [ { bounds =
                                        { x = toFloat lastCard.column * (cardWidth + padding)
                                        , y = (toFloat lastCard.row - 1) * (cardHeight + padding)
                                        , width = cardWidth + padding
                                        , height = cardHeight
                                        }
                                  , target = After lastPipeline.name
                                  }
                                ]

                        Nothing ->
                            []
                   )

        pipelineCards =
            g.pipelines
                |> List.map
                    (\pipeline ->
                        cardLookup
                            |> Dict.get pipeline.id
                            |> Maybe.withDefault { row = 0, column = 0, width = 0, height = 0 }
                            |> (\card -> ( pipeline, card ))
                    )
                |> List.filter (\( _, card ) -> isVisible card)
                |> List.map
                    (\( pipeline, card ) ->
                        { pipeline = pipeline
                        , bounds =
                            { x = (toFloat card.column - 1) * (cardWidth + padding) + padding
                            , y = (toFloat card.row - 1) * (cardHeight + padding)
                            , width =
                                cardWidth
                                    * toFloat card.width
                                    + padding
                                    * (toFloat card.width - 1)
                            , height =
                                cardHeight
                                    * toFloat card.height
                                    + padding
                                    * (toFloat card.height - 1)
                            }
                        }
                    )
    in
    { pipelineCards = pipelineCards
    , dropAreas = dropAreas
    , height = totalCardsHeight
    }
