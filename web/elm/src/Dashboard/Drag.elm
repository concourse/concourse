module Dashboard.Drag exposing (drag, dragPipeline, insertAt)

import Dashboard.Group.Models exposing (Pipeline)
import List.Extra
import Message.Message exposing (DropTarget(..))


insertAt : Int -> a -> List a -> List a
insertAt idx x xs =
    case ( idx > 0, xs ) of
        ( True, head :: tail ) ->
            head :: insertAt (idx - 1) x tail

        _ ->
            x :: xs


dragPipeline : String -> DropTarget -> List Pipeline -> List Pipeline
dragPipeline pipeline target pipelines =
    let
        pipelineIndex name =
            pipelines |> List.Extra.findIndex (.name >> (==) name)

        fromIndex =
            pipelineIndex pipeline

        toIndex =
            case target of
                Before name ->
                    pipelineIndex name

                After name ->
                    pipelineIndex name |> Maybe.map ((+) 1)
    in
    case ( fromIndex, toIndex ) of
        ( Just from, Just to ) ->
            drag from (to + 1) pipelines

        _ ->
            pipelines


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
