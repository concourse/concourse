module FlySuccess exposing (Model, Msg(..), copied, hover, init, view)

import Colors
import Html exposing (Html)
import Html.Attributes exposing (attribute, id, style)
import Html.Events exposing (onClick, onMouseEnter, onMouseLeave)
import Routes
import QueryString


type Msg
    = CopyTokenButtonHover Bool
    | CopyToken


type alias Model =
    { buttonHovered : Bool
    , tokenCopied : Bool
    , authToken : String
    }


hover : Bool -> Model -> Model
hover hoverState model =
    { model | buttonHovered = hoverState }


copied : Model -> Model
copied model =
    { model | tokenCopied = True }


init : Routes.ConcourseRoute -> Model
init route =
    { buttonHovered = False
    , tokenCopied = False
    , authToken =
        QueryString.one QueryString.string "token" route.queries
            |> Maybe.withDefault ""
    }


view : Model -> Html Msg
view { buttonHovered, tokenCopied, authToken } =
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
                        ++ if tokenCopied then
                            Colors.flySuccessTokenCopied
                           else
                            Colors.text
                  )
                , ( "margin", "10px 0" )
                , ( "padding", "10px 0" )
                , ( "width", "212px" )
                , ( "cursor"
                  , if tokenCopied then
                        "default"
                    else
                        "pointer"
                  )
                , ( "text-align", "center" )
                , ( "background-color"
                  , if tokenCopied then
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
            , attribute "data-clipboard-text" authToken
            ]
            [ Html.text <|
                if tokenCopied then
                    "token copied"
                else
                    "copy token to clipboard"
            ]
        ]
