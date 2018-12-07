module FlySuccess exposing (Model, Msg(..), click, hover, init, view)

import Colors
import Html exposing (Html)
import Html.Attributes exposing (id, style)
import Html.Events exposing (onClick, onMouseEnter, onMouseLeave)


type Msg
    = CopyTokenButtonHover Bool
    | CopyToken


type alias Model =
    { buttonHovered : Bool
    , buttonClicked : Bool
    }


hover : Bool -> Model -> Model
hover hoverState model =
    { model | buttonHovered = hoverState }


click : Model -> Model
click model =
    { model | buttonClicked = True }


init : Model
init =
    { buttonHovered = False
    , buttonClicked = False
    }


view : { buttonHovered : Bool, buttonClicked : Bool } -> Html Msg
view { buttonHovered, buttonClicked } =
    Html.div
        [ id "success-card"
        , style
            [ ( "background-color", Colors.flySuccessCard )
            , ( "padding", "30px 20px" )
            , ( "margin", "20px 30px" )
            , ( "display", "flex" )
            , ( "flex-direction", "column" )
            , ( "align-items", "flex-start" )
            ]
        ]
        [ Html.p
            [ id "success-message"
            , style
                [ ( "font-size", "18px" )
                , ( "margin", "0" )
                ]
            ]
            [ Html.text "you have successfully logged in!" ]
        , Html.p
            [ id "success-details"
            , style
                [ ( "font-size", "14px" )
                , ( "margin", "10px 0" )
                ]
            ]
            [ Html.text "return to fly OR go back to the Concourse dashboard" ]
        , Html.span
            [ id "copy-token"
            , style
                [ ( "border"
                  , "1px solid "
                        ++ if buttonClicked then
                            Colors.flySuccessTokenCopied
                           else
                            Colors.text
                  )
                , ( "margin", "10px 0" )
                , ( "padding", "10px 0" )
                , ( "width", "212px" )
                , ( "cursor"
                  , if buttonClicked then
                        "default"
                    else
                        "pointer"
                  )
                , ( "text-align", "center" )
                , ( "background-color"
                  , if buttonClicked then
                        Colors.flySuccessTokenCopied
                    else if buttonHovered then
                        Colors.flySuccessButtonHover
                    else
                        Colors.flySuccessCard
                  )
                ]
            , onMouseEnter <| CopyTokenButtonHover True
            , onMouseLeave <| CopyTokenButtonHover False
            , onClick CopyToken
            ]
            [ Html.text <|
                if buttonClicked then
                    "token copied"
                else
                    "copy token to clipboard"
            ]
        ]
