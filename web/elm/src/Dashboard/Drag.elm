module Dashboard.Drag exposing (drag, dragCardIndices, insertAt, reverseIndices)

import Dashboard.Group.Models exposing (Card(..), cardIdentifier)
import List.Extra
import Message.Message exposing (DropTarget(..))


insertAt : Int -> a -> List a -> List a
insertAt idx x xs =
    case ( idx > 0, xs ) of
        ( True, head :: tail ) ->
            head :: insertAt (idx - 1) x tail

        _ ->
            x :: xs


dragCardIndices : Int -> DropTarget -> List Card -> Maybe ( Int, Int )
dragCardIndices cardId target cards =
    let
        cardIndex id =
            cards |> List.Extra.findIndex (cardIdentifier >> (==) id)

        fromIndex =
            cardIndex cardId

        toIndex =
            (case target of
                Before name ->
                    cardIndex name

                After name ->
                    cardIndex name
                        |> Maybe.map ((+) 1)
            )
                |> Maybe.map ((+) 1)
    in
    Maybe.map2 Tuple.pair fromIndex toIndex


reverseIndices : Int -> ( Int, Int ) -> ( Int, Int )
reverseIndices length ( from, to ) =
    ( max 0 (to - 1), min length (from + 1) )


drag : Int -> Int -> List a -> List a
drag from to xs =
    if from >= to then
        let
            n =
                List.length xs
        in
        List.reverse (drag (n - from - 1) (n - to + 1) (List.reverse xs))

    else
        case xs of
            [] ->
                []

            head :: tail ->
                if from == 0 then
                    insertAt (to - 1) head tail

                else
                    head :: drag (from - 1) (to - 1) tail
