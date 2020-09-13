module Dashboard.Drag exposing (drag, dragCard, insertAt)

import Dashboard.Group.Models exposing (Card(..), cardName)
import List.Extra
import Message.Message exposing (DropTarget(..))


insertAt : Int -> a -> List a -> List a
insertAt idx x xs =
    case ( idx > 0, xs ) of
        ( True, head :: tail ) ->
            head :: insertAt (idx - 1) x tail

        _ ->
            x :: xs


dragCard : String -> DropTarget -> List Card -> List Card
dragCard card target cards =
    let
        cardIndex name =
            cards |> List.Extra.findIndex (cardName >> (==) name)

        fromIndex =
            cardIndex card

        toIndex =
            case target of
                Before name ->
                    cardIndex name

                After name ->
                    cardIndex name |> Maybe.map ((+) 1)
    in
    case ( fromIndex, toIndex ) of
        ( Just from, Just to ) ->
            drag from (to + 1) cards

        _ ->
            cards


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
