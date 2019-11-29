module Dashboard.Drag exposing (drag, insertAt)


insertAt : Int -> a -> List a -> List a
insertAt idx x xs =
    case ( idx > 0, xs ) of
        ( True, head :: tail ) ->
            head :: insertAt (idx - 1) x tail

        _ ->
            x :: xs


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
