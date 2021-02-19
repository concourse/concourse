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


carHeaderVerticalSpace : number
carHeaderVerticalSpace =
    8


cardHeaderHeight : number -> number
cardHeaderHeight numRows =
    32 + 21 * numRows + (numRows - 1) * carHeaderVerticalSpace


groupHeaderLineHeight : number
groupHeaderLineHeight =
    25


groupHeaderHeight : number
groupHeaderHeight =
    padding + groupHeaderLineHeight


sectionHeaderHeight : number
sectionHeaderHeight =
    70


sectionSpacerHeight : number
sectionSpacerHeight =
    30
