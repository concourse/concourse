module Dashboard.PipelineGrid.Layout exposing (Card, cardSize, layout)


type alias GridSpan =
    Int


type alias Card =
    { spannedColumns : GridSpan
    , spannedRows : GridSpan
    , column : Int
    , row : Int
    }


countToSpan : Int -> GridSpan
countToSpan count =
    if count > 24 then
        3

    else if count > 12 then
        2

    else
        1


cardSize : ( Int, Int ) -> ( GridSpan, GridSpan )
cardSize ( w, h ) =
    ( countToSpan w
    , countToSpan h
    )


layout : Int -> List ( GridSpan, GridSpan ) -> List Card
layout numColumns cardSizes =
    cardSizes
        |> List.foldl
            (\( w, h ) { cards, column, row, rowHeight } ->
                let
                    breaksRow =
                        (column + w > numColumns + 1)
                            && (column /= 1)

                    newColumn =
                        if breaksRow then
                            1

                        else
                            column

                    newRow =
                        if breaksRow then
                            row + rowHeight

                        else
                            row

                    newRowHeight =
                        if breaksRow then
                            h

                        else
                            max rowHeight h
                in
                { cards = { spannedColumns = w, spannedRows = h, column = newColumn, row = newRow } :: cards
                , column = newColumn + w
                , row = newRow
                , rowHeight = newRowHeight
                }
            )
            { cards = [], column = 1, row = 1, rowHeight = 1 }
        |> .cards
        |> List.reverse
