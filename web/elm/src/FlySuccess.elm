module FlySuccess exposing (Model, Msg(..), init, update, view)

import Colors
import Html exposing (Html)
import Html.Attributes exposing (attribute, id, style)
import Html.Events exposing (onClick, onMouseEnter, onMouseLeave)
import Http
import QueryString


type Msg
    = CopyTokenButtonHover Bool
    | CopyToken
    | Noop


type alias Model =
    { buttonHovered : Bool
    , tokenCopied : Bool
    , authToken : String
    }


init : { authToken : String, fly : Maybe String } -> ( Model, Cmd Msg )
init ({ authToken, fly } as params) =
    ( { buttonHovered = False
      , tokenCopied = False
      , authToken = authToken
      }
    , sendTokenToFly params
    )


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        CopyTokenButtonHover hoverState ->
            ( { model | buttonHovered = hoverState }, Cmd.none )

        CopyToken ->
            ( { model | tokenCopied = True }, Cmd.none )

        Noop ->
            ( model, Cmd.none )


sendTokenToFly : { authToken : String, fly : Maybe String } -> Cmd Msg
sendTokenToFly { authToken, fly } =
    case fly of
        Nothing ->
            Cmd.none

        Just url ->
            let
                queryString =
                    QueryString.empty
                        |> QueryString.add "token" authToken
                        |> QueryString.render
            in
                Http.getString (url ++ queryString)
                    |> Http.send (always Noop)


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
