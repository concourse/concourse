module Styles exposing (disableInteraction)


disableInteraction : List ( String, String )
disableInteraction =
    [ ( "cursor", "default" )
    , ( "user-select", "none" )
    , ( "-ms-user-select", "none" )
    , ( "-moz-user-select", "none" )
    , ( "-khtml-user-select", "none" )
    , ( "-webkit-user-select", "none" )
    , ( "-webkit-touch-callout", "none" )
    ]
