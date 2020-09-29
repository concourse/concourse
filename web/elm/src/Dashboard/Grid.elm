module Dashboard.Grid exposing
    ( Bounds
    , Card
    , DropArea
    , Header
    , computeFavoritesLayout
    , computeLayout
    )

import Concourse exposing (flattenJson)
import Dashboard.Drag as Drag
import Dashboard.Grid.Constants
    exposing
        ( cardBodyHeight
        , cardHeaderHeight
        , cardWidth
        , groupHeaderHeight
        , padding
        )
import Dashboard.Grid.Layout as Layout
import Dashboard.Group.Models as Models
    exposing
        ( Card(..)
        , cardIdentifier
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
    , headerHeight : Float
    , gridElement : Layout.GridElement
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
        dragIndices =
            case ( params.dragState, params.dropState ) of
                ( Dragging team cardId, Dropping target ) ->
                    if teamName == team then
                        Drag.dragCardIndices cardId target cards

                    else
                        Nothing

                _ ->
                    Nothing

        orderedCards =
            case dragIndices of
                Just ( from, to ) ->
                    Drag.drag from to cards

                _ ->
                    cards

        result =
            computeCards
                { colGap = padding
                , rowGap = padding
                , offsetX = padding
                , offsetY = 0
                }
                params
                orderedCards

        dropAreaBounds bounds =
            { bounds | x = bounds.x - padding, width = bounds.width + padding }

        boundsToRightOf otherBounds =
            { otherBounds
                | x = otherBounds.x + otherBounds.width
                , y = otherBounds.y
                , width = cardWidth + padding
                , height = otherBounds.height
            }

        cardDropAreas =
            result.allCards
                |> List.map2 Tuple.pair cards
                |> List.foldl
                    (\( origCard, { bounds, gridElement } as curCard ) ( dropAreas, prevDropArea ) ->
                        let
                            curBounds =
                                case prevDropArea of
                                    Just prev ->
                                        if
                                            (prev.gridElement.row < gridElement.row)
                                                && (prev.gridElement.column + prev.gridElement.spannedColumns <= result.numColumns)
                                        then
                                            boundsToRightOf prev.bounds

                                        else
                                            dropAreaBounds bounds

                                    Nothing ->
                                        dropAreaBounds bounds

                            curDropArea =
                                { bounds = curBounds, target = Before <| cardIdentifier origCard }
                        in
                        ( curDropArea :: dropAreas, Just curCard )
                    )
                    ( [], Nothing )
                |> Tuple.first
                |> List.reverse

        allDropAreas =
            cardDropAreas
                ++ (case List.Extra.last result.allCards of
                        Just { bounds, card } ->
                            [ { bounds = boundsToRightOf bounds
                              , target = After <| cardIdentifier card
                              }
                            ]

                        Nothing ->
                            []
                   )

        -- Due to a quirk with animations + Html.keyed, the cards need to remain
        -- in the same order even when we display the drag'n'drop preview - otherwise,
        -- the animation is cut short in one direction
        cardsWithOriginalOrder =
            result.allCards
                |> (case dragIndices of
                        Just idxs ->
                            Drag.reverseIndices (List.length result.allCards) idxs
                                |> (\( revFrom, revTo ) -> Drag.drag revFrom revTo)

                        _ ->
                            identity
                   )
    in
    { cards = cardsWithOriginalOrder |> List.filter (isVisible params)
    , dropAreas = allDropAreas |> List.filter (isVisible params)
    , height =
        if List.isEmpty cards then
            cardHeaderHeight 1 + cardBodyHeight + padding

        else
            result.totalHeight
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
        result =
            computeCards
                { colGap = padding
                , rowGap = groupHeaderHeight
                , offsetX = padding
                , offsetY = groupHeaderHeight
                }
                params
                cards

        headers =
            result.allCards
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
                                , y = first.bounds.y - groupHeaderHeight
                                , width = last.bounds.x + cardWidth - first.bounds.x
                                , height = groupHeaderHeight
                                }
                          }
                            :: headers_
                        )
                    )
                    ( Nothing, [] )
                |> Tuple.second
    in
    { cards = result.allCards |> List.filter (isVisible params)
    , headers = headers |> List.filter (isVisible params)
    , height = result.totalHeight
    }


cardSizes :
    Dict Concourse.DatabaseID (List (List Concourse.JobIdentifier))
    -> List Models.Card
    -> List ( Layout.GridSpan, Layout.GridSpan )
cardSizes pipelineLayers =
    List.map
        (\card ->
            case card of
                PipelineCard pipeline ->
                    Dict.get pipeline.id pipelineLayers
                        |> Maybe.withDefault []
                        |> (\layers ->
                                Layout.cardSize
                                    ( List.length layers
                                    , layers
                                        |> List.map List.length
                                        |> List.maximum
                                        |> Maybe.withDefault 0
                                    )
                           )

                InstanceGroupCard _ _ ->
                    ( 1, 1 )
        )


isVisible : { a | scrollTop : Float, viewportHeight : Float } -> { b | bounds : Bounds } -> Bool
isVisible { viewportHeight, scrollTop } { bounds } =
    let
        leeway =
            100
    in
    (bounds.y + bounds.height >= scrollTop - leeway)
        && (bounds.y <= scrollTop + viewportHeight + leeway)


cardBounds :
    { colGap : Float
    , rowGap : Float
    , offsetX : Float
    , offsetY : Float
    }
    -> Float
    -> Layout.GridElement
    -> Float
    -> Bounds
cardBounds { colGap, rowGap, offsetX, offsetY } y elem headerHeight =
    let
        colWidth =
            cardWidth + colGap
    in
    { x = (toFloat elem.column - 1) * colWidth + offsetX
    , y = y + offsetY
    , width =
        cardWidth
            * toFloat elem.spannedColumns
            + colGap
            * (toFloat elem.spannedColumns - 1)
    , height =
        headerHeight
            + (cardBodyHeight * toFloat elem.spannedRows)
            + (rowGap * (toFloat elem.spannedRows - 1))
    }


computeCards :
    { colGap : Float
    , rowGap : Float
    , offsetX : Float
    , offsetY : Float
    }
    ->
        { a
            | viewportWidth : Float
            , pipelineLayers : Dict Concourse.DatabaseID (List (List Concourse.JobIdentifier))
        }
    -> List Models.Card
    ->
        { totalHeight : Float
        , allCards : List Card
        , numColumns : Int
        }
computeCards config params cards =
    let
        numColumns =
            max 1 (floor (params.viewportWidth / (cardWidth + padding)))

        gridElements =
            cards
                |> cardSizes params.pipelineLayers
                |> Layout.layout numColumns
    in
    cards
        |> List.map2
            (\gridElement card ->
                { gridElement = gridElement
                , card = card
                }
            )
            gridElements
        |> List.Extra.groupWhile (\a b -> a.gridElement.row == b.gridElement.row)
        |> List.foldl
            (\( first, rest ) state ->
                let
                    headerHeight =
                        maxBy (numHeaderRows << .card) first rest
                            |> cardHeaderHeight
                            |> toFloat

                    curCards =
                        (first :: rest)
                            |> List.map
                                (\{ card, gridElement } ->
                                    { card = card
                                    , headerHeight = headerHeight
                                    , gridElement = gridElement
                                    , bounds = cardBounds config state.totalHeight gridElement headerHeight
                                    }
                                )

                    curRowHeight =
                        curCards
                            |> List.map (\{ bounds } -> bounds.height + config.rowGap)
                            |> List.maximum
                            |> Maybe.withDefault 0
                in
                { state
                    | totalHeight = state.totalHeight + curRowHeight
                    , allCards = state.allCards ++ curCards
                }
            )
            { totalHeight = 0
            , allCards = []
            , numColumns = numColumns
            }


numHeaderRows : Models.Card -> Int
numHeaderRows card =
    case card of
        Models.PipelineCard pipeline ->
            if Dict.isEmpty pipeline.instanceVars then
                1

            else
                pipeline.instanceVars
                    |> Dict.toList
                    |> List.concatMap (\( k, v ) -> flattenJson k v)
                    |> List.length

        Models.InstanceGroupCard _ _ ->
            1


maxBy : (a -> comparable) -> a -> List a -> comparable
maxBy fn first rest =
    case List.map fn rest |> List.maximum of
        Nothing ->
            fn first

        Just restMax ->
            max (fn first) restMax
