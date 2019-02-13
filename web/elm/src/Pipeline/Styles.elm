module Pipeline.Styles exposing
    ( groupItem
    , groupsBar
    , groupsList
    )

import Colors


groupsBar : List ( String, String )
groupsBar =
    [ ( "background-color", Colors.groupsBarBackground )
    , ( "color", Colors.dashboardText )
    , ( "margin-top", "54px" )
    ]


groupsList : List ( String, String )
groupsList =
    [ ( "flex-grow", "1" )
    , ( "display", "flex" )
    , ( "flex-flow", "row wrap" )
    , ( "padding", "5px" )
    , ( "list-style", "none" )
    ]


groupItem : Bool -> List ( String, String )
groupItem selected =
    [ ( "font-size", "18px" )
    , ( "background", "rgba(151, 151, 151, 0.1)" )
    , ( "margin", "5px" )
    , ( "padding", "10px" )
    ]
        ++ (if selected then
                [ ( "opacity", "1" )
                , ( "border", "1px solid #979797" )
                ]

            else
                [ ( "opacity", "0.6" )
                , ( "border", "1px solid #2b2a2a" )
                ]
           )
