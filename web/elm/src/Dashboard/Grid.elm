module Dashboard.Grid exposing
    ( Bounds
    , Card
    , DropArea
    , Header
    , computeFavoritesLayout
    , computeLayout
    )

import Concourse
import Dashboard.Drag exposing (dragCard)
import Dashboard.Grid.Constants
    exposing
        ( cardHeight
        , cardWidth
        , headerHeight
        , padding
        )
import Dashboard.Grid.Layout as Layout
import Dashboard.Group.Models as Models
    exposing
        ( Card(..)
        , Pipeline
        , cardName
        , cardTeamName
        )
import Dashboard.Models exposing (DragState(..), DropState(..))
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


type alias Card =
    { bounds : Bounds
    , card : Models.Card
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
    -> Concourse.TeamName
    -> List Models.Card
    ->
        { cards : List Card
        , dropAreas : List DropArea
        , height : Float
        }
computeLayout params teamName cards =
    let
        orderedCards =
            case ( params.dragState, params.dropState ) of
                ( Dragging team pipeline, Dropping target ) ->
                    if teamName == team then
                        dragCard pipeline target cards

                    else
                        cards

                _ ->
                    cards

        numColumns =
            max 1 (floor (params.viewportWidth / (cardWidth + padding)))

        rowHeight =
            cardHeight + padding

        isVisible_ =
            isVisible params.viewportHeight params.scrollTop rowHeight

        gridElements =
            orderedCards
                |> previewSizes params.pipelineLayers
                |> List.map Layout.cardSize
                |> Layout.layout numColumns

        numRows =
            gridElements
                |> List.map (\c -> c.row + c.spannedRows - 1)
                |> List.maximum
                |> Maybe.withDefault 1

        totalCardsHeight =
            toFloat numRows
                * cardHeight
                + padding
                * toFloat numRows

        gridElementLookup =
            gridElements
                |> List.map2 Tuple.pair orderedCards
                |> List.map (\( card, gridElement ) -> ( cardName card, gridElement ))
                |> Dict.fromList

        prevAndCurrentGridElement =
            gridElements
                |> List.map2 Tuple.pair (Nothing :: (gridElements |> List.map Just))

        cardBounds =
            boundsForGridElement
                { colGap = padding
                , rowGap = padding
                , offsetX = padding
                , offsetY = 0
                }

        dropAreaBounds =
            cardBounds >> (\b -> { b | x = b.x - padding, width = b.width + padding })

        dropAreas =
            (prevAndCurrentGridElement
                |> List.map2 Tuple.pair cards
                |> List.filter (\( _, ( _, gridElement ) ) -> isVisible_ gridElement)
                |> List.map
                    (\( card, ( prevGridElement, gridElement ) ) ->
                        let
                            boundsToRightOf otherCard =
                                dropAreaBounds
                                    { otherCard
                                        | column = otherCard.column + otherCard.spannedColumns
                                        , spannedColumns = 1
                                    }

                            bounds =
                                case prevGridElement of
                                    Just otherGridElement ->
                                        if
                                            (otherGridElement.row < gridElement.row)
                                                && (otherGridElement.column + otherGridElement.spannedColumns <= numColumns)
                                        then
                                            boundsToRightOf otherGridElement

                                        else
                                            dropAreaBounds gridElement

                                    Nothing ->
                                        dropAreaBounds gridElement
                        in
                        { bounds = bounds, target = Before <| cardName card }
                    )
            )
                ++ (case List.head (List.reverse (List.map2 Tuple.pair gridElements cards)) of
                        Just ( lastGridElement, lastCard ) ->
                            if not (isVisible_ lastGridElement) then
                                []

                            else
                                [ { bounds =
                                        dropAreaBounds
                                            { lastGridElement
                                                | column = lastGridElement.column + lastGridElement.spannedColumns
                                                , spannedColumns = 1
                                            }
                                  , target = After <| cardName lastCard
                                  }
                                ]

                        Nothing ->
                            []
                   )

        gridCards =
            cards
                |> List.map
                    (\card ->
                        gridElementLookup
                            |> Dict.get (cardName card)
                            |> Maybe.withDefault
                                { row = 0
                                , column = 0
                                , spannedColumns = 0
                                , spannedRows = 0
                                }
                            |> (\gridElement -> ( card, gridElement ))
                    )
                |> List.filter (\( _, gridElement ) -> isVisible_ gridElement)
                |> List.map
                    (\( card, gridElement ) ->
                        { card = card
                        , bounds = cardBounds gridElement
                        }
                    )
    in
    { cards = gridCards
    , dropAreas = dropAreas
    , height = totalCardsHeight
    }


computeFavoritesLayout :
    { pipelineLayers : Dict Concourse.DatabaseID (List (List Concourse.JobIdentifier))
    , viewportWidth : Float
    , viewportHeight : Float
    , scrollTop : Float
    }
    -> List Models.Card
    ->
        { cards : List Card
        , headers : List Header
        , height : Float
        }
computeFavoritesLayout params cards =
    let
        numColumns =
            max 1 (floor (params.viewportWidth / (cardWidth + padding)))

        rowHeight =
            cardHeight + headerHeight

        isVisible_ =
            isVisible params.viewportHeight params.scrollTop rowHeight

        gridElements =
            cards
                |> previewSizes params.pipelineLayers
                |> List.map Layout.cardSize
                |> Layout.layout numColumns

        numRows =
            gridElements
                |> List.map (\c -> c.row + c.spannedRows - 1)
                |> List.maximum
                |> Maybe.withDefault 1

        totalCardsHeight =
            toFloat numRows * rowHeight

        cardBounds =
            boundsForGridElement
                { colGap = padding
                , rowGap = headerHeight
                , offsetX = padding
                , offsetY = headerHeight
                }

        favCards =
            gridElements
                |> List.map2 Tuple.pair cards
                |> List.filter (\( _, gridElement ) -> isVisible_ gridElement)
                |> List.map
                    (\( card, gridElement ) ->
                        { card = card
                        , bounds = cardBounds gridElement
                        }
                    )

        headers =
            favCards
                |> List.Extra.groupWhile
                    (\c1 c2 ->
                        (cardTeamName c1.card == cardTeamName c2.card)
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
                                cardTeamName first.card

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
    { cards = favCards
    , headers = headers
    , height = totalCardsHeight
    }


previewSizes :
    Dict Concourse.DatabaseID (List (List Concourse.JobIdentifier))
    -> List Models.Card
    -> List ( Int, Int )
previewSizes pipelineLayers =
    List.map
        (\card ->
            case card of
                PipelineCard pipeline ->
                    Dict.get pipeline.id pipelineLayers
                        |> Maybe.withDefault []
                        |> (\layers ->
                                ( List.length layers
                                , layers
                                    |> List.map List.length
                                    |> List.maximum
                                    |> Maybe.withDefault 0
                                )
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


boundsForGridElement :
    { colGap : Float
    , rowGap : Float
    , offsetX : Float
    , offsetY : Float
    }
    -> Layout.GridElement
    -> Bounds
boundsForGridElement { colGap, rowGap, offsetX, offsetY } elem =
    let
        colWidth =
            cardWidth + colGap

        rowHeight =
            cardHeight + rowGap
    in
    { x = (toFloat elem.column - 1) * colWidth + offsetX
    , y = (toFloat elem.row - 1) * rowHeight + offsetY
    , width =
        cardWidth
            * toFloat elem.spannedColumns
            + colGap
            * (toFloat elem.spannedColumns - 1)
    , height =
        cardHeight
            * toFloat elem.spannedRows
            + rowGap
            * (toFloat elem.spannedRows - 1)
    }
