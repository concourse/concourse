module Dashboard.PipelineGrid exposing
    ( Bounds
    , DropArea
    , Header
    , PipelineCard
    , computeFavoritePipelinesLayout
    , computeLayout
    )

import Concourse
import Dashboard.Drag exposing (dragPipeline)
import Dashboard.Group.Models exposing (Group, Pipeline)
import Dashboard.Models exposing (DragState(..), DropState(..))
import Dashboard.PipelineGrid.Constants
    exposing
        ( cardHeight
        , cardWidth
        , headerHeight
        , padding
        )
import Dashboard.PipelineGrid.Layout as Layout
import Dict exposing (Dict)
import List.Extra
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


type alias Header =
    { bounds : Bounds
    , header : String
    }


computeLayout :
    { dragState : DragState
    , dropState : DropState
    , pipelineLayers : Dict Concourse.DatabaseID (List (List Concourse.JobIdentifier))
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

        rowHeight =
            cardHeight + padding

        isVisible_ =
            isVisible params.viewportHeight params.scrollTop rowHeight

        cards =
            orderedPipelines
                |> previewSizes params.pipelineLayers
                |> List.map Layout.cardSize
                |> Layout.layout numColumns

        numRows =
            cards
                |> List.map (\c -> c.row + c.spannedRows - 1)
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

        cardBounds =
            boundsForCell
                { colGap = padding
                , rowGap = padding
                , offsetX = padding
                , offsetY = 0
                }

        dropAreaBounds =
            cardBounds >> (\b -> { b | x = b.x - padding, width = b.width + padding })

        dropAreas =
            (prevAndCurrentCards
                |> List.map2 Tuple.pair g.pipelines
                |> List.filter (\( _, ( _, card ) ) -> isVisible_ card)
                |> List.map
                    (\( pipeline, ( prevCard, card ) ) ->
                        let
                            boundsToRightOf otherCard =
                                dropAreaBounds
                                    { otherCard
                                        | column = otherCard.column + otherCard.spannedColumns
                                        , spannedColumns = 1
                                    }

                            bounds =
                                case prevCard of
                                    Just otherCard ->
                                        if
                                            (otherCard.row < card.row)
                                                && (otherCard.column + otherCard.spannedColumns <= numColumns)
                                        then
                                            boundsToRightOf otherCard

                                        else
                                            dropAreaBounds card

                                    Nothing ->
                                        dropAreaBounds card
                        in
                        { bounds = bounds, target = Before pipeline.name }
                    )
            )
                ++ (case List.head (List.reverse (List.map2 Tuple.pair cards g.pipelines)) of
                        Just ( lastCard, lastPipeline ) ->
                            if not (isVisible_ lastCard) then
                                []

                            else
                                [ { bounds =
                                        dropAreaBounds
                                            { lastCard
                                                | column = lastCard.column + lastCard.spannedColumns
                                                , spannedColumns = 1
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
                            |> Maybe.withDefault
                                { row = 0
                                , column = 0
                                , spannedColumns = 0
                                , spannedRows = 0
                                }
                            |> (\card -> ( pipeline, card ))
                    )
                |> List.filter (\( _, card ) -> isVisible_ card)
                |> List.map
                    (\( pipeline, card ) ->
                        { pipeline = pipeline
                        , bounds = cardBounds card
                        }
                    )
    in
    { pipelineCards = pipelineCards
    , dropAreas = dropAreas
    , height = totalCardsHeight
    }


computeFavoritePipelinesLayout :
    { pipelineLayers : Dict Concourse.DatabaseID (List (List Concourse.JobIdentifier))
    , viewportWidth : Float
    , viewportHeight : Float
    , scrollTop : Float
    }
    -> List Pipeline
    ->
        { pipelineCards : List PipelineCard
        , headers : List Header
        , height : Float
        }
computeFavoritePipelinesLayout params pipelines =
    let
        numColumns =
            max 1 (floor (params.viewportWidth / (cardWidth + padding)))

        rowHeight =
            cardHeight + headerHeight

        isVisible_ =
            isVisible params.viewportHeight params.scrollTop rowHeight

        cards =
            pipelines
                |> previewSizes params.pipelineLayers
                |> List.map Layout.cardSize
                |> Layout.layout numColumns

        numRows =
            cards
                |> List.map (\c -> c.row + c.spannedRows - 1)
                |> List.maximum
                |> Maybe.withDefault 1

        totalCardsHeight =
            toFloat numRows * rowHeight

        cardBounds =
            boundsForCell
                { colGap = padding
                , rowGap = headerHeight
                , offsetX = padding
                , offsetY = headerHeight
                }

        pipelineCards =
            cards
                |> List.map2 Tuple.pair pipelines
                |> List.filter (\( _, card ) -> isVisible_ card)
                |> List.map
                    (\( pipeline, card ) ->
                        { pipeline = pipeline
                        , bounds = cardBounds card
                        }
                    )

        headers =
            pipelineCards
                |> List.Extra.groupWhile
                    (\c1 c2 ->
                        (c1.pipeline.teamName == c2.pipeline.teamName)
                            && (c1.bounds.y == c2.bounds.y)
                    )
                |> List.map
                    (\( c, cs ) ->
                        ( c
                        , case List.Extra.last cs of
                            Nothing ->
                                c

                            Just tail ->
                                tail
                        )
                    )
                |> List.foldl
                    (\( first, last ) ( prevTeam, headers_ ) ->
                        let
                            curTeam =
                                first.pipeline.teamName

                            header =
                                case prevTeam of
                                    Nothing ->
                                        curTeam

                                    Just prevTeam_ ->
                                        if prevTeam_ == curTeam then
                                            curTeam ++ " (continued)"

                                        else
                                            curTeam
                        in
                        ( Just curTeam
                        , { header = header
                          , bounds =
                                { x = first.bounds.x
                                , y = first.bounds.y - headerHeight
                                , width = last.bounds.x + cardWidth - first.bounds.x
                                , height = headerHeight
                                }
                          }
                            :: headers_
                        )
                    )
                    ( Nothing, [] )
                |> Tuple.second
    in
    { pipelineCards = pipelineCards
    , headers = headers
    , height = totalCardsHeight
    }


previewSizes :
    Dict Concourse.DatabaseID (List (List Concourse.JobIdentifier))
    -> List Pipeline
    -> List ( Int, Int )
previewSizes pipelineLayers =
    List.map
        (\pipeline ->
            Dict.get pipeline.id pipelineLayers
                |> Maybe.withDefault []
        )
        >> List.map
            (\layers ->
                ( List.length layers
                , layers
                    |> List.map List.length
                    |> List.maximum
                    |> Maybe.withDefault 0
                )
            )


isVisible : Float -> Float -> Float -> { r | row : Int, spannedRows : Int } -> Bool
isVisible viewportHeight scrollTop rowHeight { row, spannedRows } =
    let
        numRowsVisible =
            ceiling (viewportHeight / rowHeight) + 1

        numRowsOffset =
            floor (scrollTop / rowHeight)
    in
    (numRowsOffset < row + spannedRows)
        && (row <= numRowsOffset + numRowsVisible)


boundsForCell :
    { colGap : Float
    , rowGap : Float
    , offsetX : Float
    , offsetY : Float
    }
    -> Layout.Card
    -> Bounds
boundsForCell { colGap, rowGap, offsetX, offsetY } card =
    let
        colWidth =
            cardWidth + colGap

        rowHeight =
            cardHeight + rowGap
    in
    { x = (toFloat card.column - 1) * colWidth + offsetX
    , y = (toFloat card.row - 1) * rowHeight + offsetY
    , width =
        cardWidth
            * toFloat card.spannedColumns
            + colGap
            * (toFloat card.spannedColumns - 1)
    , height =
        cardHeight
            * toFloat card.spannedRows
            + rowGap
            * (toFloat card.spannedRows - 1)
    }
