--  Copyright (c) 2014 The Polymer Project Authors. All rights reserved.
--  This code may only be used under the BSD style license found at http://polymer.github.io/LICENSE.txt
--  The complete set of authors may be found at http://polymer.github.io/AUTHORS.txt
--  The complete set of contributors may be found at http://polymer.github.io/CONTRIBUTORS.txt
--  Code distributed by Google as part of the polymer project is also
--  subject to an additional IP rights grant found at http://polymer.github.io/PATENTS.txt


module Views.Spinner exposing (hoverableSpinner, spinner)

import Html exposing (Html)
import Html.Attributes exposing (style)
import Html.Events exposing (onMouseEnter, onMouseLeave)
import Message.Message exposing (DomID, Message(..))


spinner : { sizePx : Float, margin : String } -> Html Message
spinner { sizePx, margin } =
    hoverableSpinner { sizePx = sizePx, margin = margin, hoverable = Nothing }


hoverableSpinner :
    { sizePx : Float, margin : String, hoverable : Maybe DomID }
    -> Html Message
hoverableSpinner { sizePx, margin, hoverable } =
    Html.div
        -- preloader-wrapper active
        ([ style "width" <| String.fromFloat sizePx ++ "px"
         , style "height" <| String.fromFloat sizePx ++ "px"
         , style "box-sizing" "border-box"
         , style "animation" "container-rotate 1568ms linear infinite"
         , style "margin" margin
         ]
            ++ (case hoverable of
                    Just h ->
                        [ onMouseEnter <| Hover <| Just h
                        , onMouseLeave <| Hover Nothing
                        ]

                    Nothing ->
                        []
               )
        )
        [ Html.div
            -- spinner-layer spinner-blue-only
            [ style "height" "100%"
            , style "border-color" "white"
            , style "animation" "fill-unfill-rotate 5332ms cubic-bezier(0.4, 0.0, 0.2, 1) infinite both"
            ]
            [ Html.div
                -- circle-clipper left
                [ style "position" "relative"
                , style "width" "50%"
                , style "height" "100%"
                , style "overflow" "hidden"
                , style "border-color" "inherit"
                , style "float" "left"
                ]
                [ Html.div
                    -- circle
                    [ style "width" "200%"
                    , style "border-width" "2px"
                    , style "box-sizing" "border-box"
                    , style "border-style" "solid"
                    , style "border-color" "inherit"
                    , style "border-bottom-color" "transparent"
                    , style "border-radius" "50%"
                    , style "position" "absolute"
                    , style "top" "0"
                    , style "bottom" "0"
                    , style "left" "0"
                    , style "border-right-color" "transparent"
                    , style "transform" "rotate(129deg)"
                    , style "animation" "left-spin 1333ms cubic-bezier(0.4, 0.0, 0.2, 1) infinite both"
                    ]
                    []
                ]
            , Html.div
                -- circle-clipper right
                [ style "position" "relative"
                , style "width" "50%"
                , style "height" "100%"
                , style "overflow" "hidden"
                , style "border-color" "inherit"
                , style "float" "right"
                ]
                [ Html.div
                    -- circle
                    [ style "width" "200%"
                    , style "border-width" "2px"
                    , style "box-sizing" "border-box"
                    , style "border-style" "solid"
                    , style "border-color" "inherit"
                    , style "border-bottom-color" "transparent"
                    , style "border-radius" "50%"
                    , style "position" "absolute"
                    , style "top" "0"
                    , style "bottom" "0"
                    , style "left" "-100%"
                    , style "border-left-color" "transparent"
                    , style "transform" "rotate(-129deg)"
                    , style "animation" "right-spin 1333ms cubic-bezier(0.4, 0.0, 0.2, 1) infinite both"
                    ]
                    []
                ]
            ]
        ]
