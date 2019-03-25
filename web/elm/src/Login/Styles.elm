module Login.Styles exposing
    ( loginComponent
    , loginContainer
    , loginItem
    , loginText
    , logoutButton
    )

import Colors


loginComponent : List ( String, String )
loginComponent =
    [ ( "max-width", "20%" ) ]


loginContainer : Bool -> List ( String, String )
loginContainer isPaused =
    [ ( "position", "relative" )
    , ( "display", "flex" )
    , ( "flex-direction", "column" )
    , ( "border-left"
      , "1px solid "
            ++ (if isPaused then
                    Colors.pausedTopbarSeparator

                else
                    Colors.background
               )
      )
    , ( "line-height", "54px" )
    ]


loginItem : List ( String, String )
loginItem =
    [ ( "padding", "0 30px" )
    , ( "cursor", "pointer" )
    , ( "display", "flex" )
    , ( "align-items", "center" )
    , ( "justify-content", "center" )
    , ( "flex-grow", "1" )
    ]


loginText : List ( String, String )
loginText =
    [ ( "overflow", "hidden" )
    , ( "text-overflow", "ellipsis" )
    ]


logoutButton : List ( String, String )
logoutButton =
    [ ( "position", "absolute" )
    , ( "top", "55px" )
    , ( "background-color", Colors.frame )
    , ( "height", "54px" )
    , ( "width", "100%" )
    , ( "border-top", "1px solid " ++ Colors.background )
    , ( "cursor", "pointer" )
    , ( "display", "flex" )
    , ( "align-items", "center" )
    , ( "justify-content", "center" )
    , ( "flex-grow", "1" )
    ]
