--  Copyright (c) 2014 The Polymer Project Authors. All rights reserved.
--  This code may only be used under the BSD style license found at http://polymer.github.io/LICENSE.txt
--  The complete set of authors may be found at http://polymer.github.io/AUTHORS.txt
--  The complete set of contributors may be found at http://polymer.github.io/CONTRIBUTORS.txt
--  Code distributed by Google as part of the polymer project is also
--  subject to an additional IP rights grant found at http://polymer.github.io/PATENTS.txt


module Spinner exposing (spinner)

import Html exposing (Html)
import Html.Attributes exposing (style)


spinner : String -> List (Html.Attribute msg) -> Html msg
spinner size attrs =
    Html.div
        -- preloader-wrapper active
        ([ style
            [ ( "width", size )
            , ( "height", size )
            , ( "box-sizing", "border-box" )
            , ( "animation"
              , "container-rotate 1568ms linear infinite"
              )
            ]
         ]
            ++ attrs
        )
        [ Html.div
            -- spinner-layer spinner-blue-only
            [ style
                [ ( "height", "100%" )
                , ( "border-color", "white" )
                , ( "animation"
                  , "fill-unfill-rotate 5332ms cubic-bezier(0.4, 0.0, 0.2, 1) infinite both"
                  )
                ]
            ]
            [ Html.div
                -- circle-clipper left
                [ style
                    [ ( "position", "relative" )
                    , ( "width", "50%" )
                    , ( "height", "100%" )
                    , ( "overflow", "hidden" )
                    , ( "border-color", "inherit" )
                    , ( "float", "left" )
                    ]
                ]
                [ Html.div
                    -- circle
                    [ style
                        [ ( "width", "200%" )
                        , ( "border-width", "2px" )
                        , ( "box-sizing", "border-box" )
                        , ( "border-style", "solid" )
                        , ( "border-color", "inherit" )
                        , ( "border-bottom-color", "transparent" )
                        , ( "border-radius", "50%" )
                        , ( "position", "absolute" )
                        , ( "top", "0" )
                        , ( "bottom", "0" )
                        , ( "left", "0" )
                        , ( "border-right-color", "transparent" )
                        , ( "transform", "rotate(129deg)" )
                        , ( "animation"
                          , "left-spin 1333ms cubic-bezier(0.4, 0.0, 0.2, 1) infinite both"
                          )
                        ]
                    ]
                    []
                ]
            , Html.div
                -- circle-clipper right
                [ style
                    [ ( "position", "relative" )
                    , ( "width", "50%" )
                    , ( "height", "100%" )
                    , ( "overflow", "hidden" )
                    , ( "border-color", "inherit" )
                    , ( "float", "right" )
                    ]
                ]
                [ Html.div
                    -- circle
                    [ style
                        [ ( "width", "200%" )
                        , ( "border-width", "2px" )
                        , ( "box-sizing", "border-box" )
                        , ( "border-style", "solid" )
                        , ( "border-color", "inherit" )
                        , ( "border-bottom-color", "transparent" )
                        , ( "border-radius", "50%" )
                        , ( "position", "absolute" )
                        , ( "top", "0" )
                        , ( "bottom", "0" )
                        , ( "left", "-100%" )
                        , ( "border-left-color", "transparent" )
                        , ( "transform", "rotate(-129deg)" )
                        , ( "animation"
                          , "right-spin 1333ms cubic-bezier(0.4, 0.0, 0.2, 1) infinite both"
                          )
                        ]
                    ]
                    []
                ]
            ]
        ]
