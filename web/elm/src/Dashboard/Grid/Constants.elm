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
    32


cardHeaderHeight : number -> number
cardHeaderHeight numRows =
    32 + 21 * numRows



-- padding top 32 + line height 25 + spacing bottom 32


groupHeaderHeight : number
groupHeaderHeight =
    89


sectionHeaderHeight : number
sectionHeaderHeight =
    70


sectionSpacerHeight : number
sectionSpacerHeight =
    30
