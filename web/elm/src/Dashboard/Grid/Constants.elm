module Dashboard.Grid.Constants exposing
    ( cardBodyHeight
    , cardHeaderHeight
    , cardWidth
    , groupHeaderHeight
    , padding
    , sectionHeaderHeight
    , sectionSpacerHeight
    )


cardWidth : number
cardWidth =
    272


cardBodyHeight : number
cardBodyHeight =
    268 - cardHeaderHeight 1


padding : number
padding =
    25


cardHeaderHeight : number -> number
cardHeaderHeight numRows =
    20 + 30 * numRows


groupHeaderHeight : number
groupHeaderHeight =
    60


sectionHeaderHeight : number
sectionHeaderHeight =
    70


sectionSpacerHeight : number
sectionSpacerHeight =
    30
