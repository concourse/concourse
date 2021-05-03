module Dashboard.Grid exposing
    ( Bounds
    , Card
    , DropArea
    , Header
    , computeFavoritesLayout
    , computeLayout
    )

import Concourse
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
        , cardName
        , cardTeamName
        )
import Dashboard.Models exposing (DragState(..), DropState(..))
import Dashboard.Pipeline as Pipeline
import Dict exposing (Dict)
import List.Extra
import Message.Message
    exposing
        ( DomID(..)
        , DropTarget(..)
        , Message(..)
        , PipelinesSection(..)
        )
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
    , pipelineLayers : Dict Concourse.DatabaseID (List (List Concourse.JobName))
    , viewportWidth : Float
    , viewportHeight : Float
    , scrollTop : Float
    , viewingInstanceGroups : Bool
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
                ( Dragging card, Dropping target ) ->
                    if teamName == cardTeamName card then
                        Drag.dragCardIndices card target cards

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
                AllPipelinesSection
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
                                { bounds = curBounds, target = Before origCard }
                        in
                        ( curDropArea :: dropAreas, Just curCard )
                    )
                    ( [], Nothing )
                |> Tuple.first
                |> List.reverse

        allDropAreas =
            cardDropAreas
                ++ (case List.Extra.last result.allCards of
                        Just { bounds } ->
                            [ { bounds = boundsToRightOf bounds
                              , target = End
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
    { pipelineLayers : Dict Concourse.DatabaseID (List (List Concourse.JobName))
    , viewportWidth : Float
    , viewportHeight : Float
    , scrollTop : Float
    , viewingInstanceGroups : Bool
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
                , rowGap = groupHeaderHeight + padding
                , offsetX = padding
                , offsetY = groupHeaderHeight
                }
                FavoritesSection
                params
                cards

        headers =
            let
                cardHeader =
                    composeHeader params.viewingInstanceGroups
            in
            result.allCards
                |> List.Extra.groupWhile
                    (\c1 c2 ->
                        (cardHeader c1.card == cardHeader c2.card)
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
                    (\( first, last ) ( prevHeader, headers_ ) ->
                        let
                            curHeader =
                                cardHeader first.card

                            header =
                                case prevHeader of
                                    Nothing ->
                                        curHeader

                                    Just prevHeader_ ->
                                        if prevHeader_ == curHeader then
                                            curHeader ++ " (continued)"

                                        else
                                            curHeader
                        in
                        ( Just curHeader
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


composeHeader : Bool -> Models.Card -> String
composeHeader viewingInstanceGroups card =
    if viewingInstanceGroups then
        cardTeamName card ++ " / " ++ cardName card

    else
        cardTeamName card


cardSizes :
    Dict Concourse.DatabaseID (List (List Concourse.JobName))
    -> List Models.Card
    -> List ( Layout.GridSpan, Layout.GridSpan )
cardSizes pipelineLayers =
    List.map
        (\card ->
            let
                pipelineCardSize pipeline =
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
            in
            case card of
                PipelineCard pipeline ->
                    pipelineCardSize pipeline

                InstancedPipelineCard pipeline ->
                    pipelineCardSize pipeline

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
    -> PipelinesSection
    ->
        { a
            | viewportWidth : Float
            , pipelineLayers : Dict Concourse.DatabaseID (List (List Concourse.JobName))
            , viewingInstanceGroups : Bool
        }
    -> List Models.Card
    ->
        { totalHeight : Float
        , allCards : List Card
        , numColumns : Int
        }
computeCards config section params cards =
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
                        maxBy (numHeaderRows section params.viewingInstanceGroups << .card) first rest
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


numHeaderRows : PipelinesSection -> Bool -> Models.Card -> Int
numHeaderRows section viewingInstanceGroups card =
    case card of
        Models.PipelineCard p ->
            List.length <| Pipeline.headerRows section viewingInstanceGroups p False

        Models.InstancedPipelineCard p ->
            List.length <| Pipeline.headerRows section viewingInstanceGroups p True

        Models.InstanceGroupCard _ _ ->
            1


maxBy : (a -> comparable) -> a -> List a -> comparable
maxBy fn first rest =
    case List.map fn rest |> List.maximum of
        Nothing ->
            fn first

        Just restMax ->
            max (fn first) restMax
