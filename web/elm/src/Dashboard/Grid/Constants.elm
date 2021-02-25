module Dashboard.Grid.Constants exposing
    ( cardBodyHeight
    , cardHeaderHeight
    , cardHeaderPadding
    , cardHeaderRowGap
    , cardHeaderRowLineHeight
    , cardWidth
    , groupHeaderHeight
    , groupHeaderLineHeight
    , padding
    , sectionHeaderHeight
    )


cardWidth : number
cardWidth =
    272


cardBodyHeight : number
cardBodyHeight =
    268 - cardHeaderHeight 1


padding : number
padding =
    32


cardHeaderRowGap : number
cardHeaderRowGap =
    8


cardHeaderPadding : number
cardHeaderPadding =
    16


cardHeaderRowLineHeight : number
cardHeaderRowLineHeight =
    21


cardHeaderHeight : number -> number
cardHeaderHeight numRows =
    2 * cardHeaderPadding + cardHeaderRowLineHeight * numRows + (numRows - 1) * cardHeaderRowGap


groupHeaderLineHeight : number
groupHeaderLineHeight =
    25


groupHeaderHeight : number
groupHeaderHeight =
    padding + groupHeaderLineHeight


sectionHeaderLineHeight : number
sectionHeaderLineHeight =
    30


sectionHeaderHeight : number
sectionHeaderHeight =
    sectionHeaderLineHeight + 2 * padding
