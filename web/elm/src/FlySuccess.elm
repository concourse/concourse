module FlySuccess exposing (Model, Msg(..), init, update, view)

import FlySuccess.Models
    exposing
        ( ButtonState(..)
        , TokenTransfer
        , TransferResult
        , hover
        , isClicked
        , isPending
        )
import FlySuccess.Styles as Styles
import FlySuccess.Text as Text
import Html exposing (Html)
import Html.Attributes exposing (attribute, id, style)
import Html.Events exposing (onClick, onMouseEnter, onMouseLeave)
import Http
import QueryString


type Msg
    = CopyTokenButtonHover Bool
    | CopyToken
    | TokenSentToFly Bool


type alias Model =
    { buttonState : ButtonState
    , authToken : String
    , tokenTransfer : TokenTransfer
    }


init : { authToken : String, flyPort : Maybe Int } -> ( Model, Cmd Msg )
init ({ authToken, flyPort } as params) =
    ( { buttonState = Unhovered
      , authToken = authToken
      , tokenTransfer =
            case flyPort of
                Just _ ->
                    Nothing

                Nothing ->
                    Just <| Err ()
      }
    , sendTokenToFly params
    )


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        CopyTokenButtonHover hovered ->
            ( { model | buttonState = hover hovered model.buttonState }
            , Cmd.none
            )

        CopyToken ->
            ( { model | buttonState = Clicked }, Cmd.none )

        TokenSentToFly success ->
            ( { model | tokenTransfer = Just <| Ok success }, Cmd.none )


sendTokenToFly : { authToken : String, flyPort : Maybe Int } -> Cmd Msg
sendTokenToFly { authToken, flyPort } =
    case flyPort of
        Nothing ->
            Cmd.none

        Just fp ->
            let
                queryString =
                    QueryString.empty
                        |> QueryString.add "token" authToken
                        |> QueryString.render
            in
            Http.request
                { method = "GET"
                , headers = []
                , url = "http://127.0.0.1:" ++ toString fp ++ queryString
                , body = Http.emptyBody
                , expect = Http.expectStringResponse (\_ -> Ok ())
                , timeout = Nothing
                , withCredentials = False
                }
                |> Http.send (\r -> TokenSentToFly (r == Ok ()))


view : Model -> Html Msg
view model =
    Html.div
        [ id "success-card"
        , style Styles.card
        ]
        [ Html.p
            [ id "success-card-title"
            , style Styles.title
            ]
            [ Html.text Text.title ]
        , Html.div
            [ id "success-card-body"
            , style Styles.body
            ]
          <|
            body model
        ]


body : Model -> List (Html Msg)
body model =
    let
        elemList =
            List.filter Tuple.second >> List.map Tuple.first
    in
    case model.tokenTransfer of
        Nothing ->
            [ Html.text Text.pending ]

        Just result ->
            let
                success =
                    result == Ok True
            in
            elemList
                [ ( paragraph
                        { identifier = "first-paragraph"
                        , lines = Text.firstParagraph success
                        }
                  , True
                  )
                , ( button model, not success )
                , ( paragraph
                        { identifier = "second-paragraph"
                        , lines = Text.secondParagraph result
                        }
                  , True
                  )
                ]


paragraph : { identifier : String, lines : Text.Paragraph } -> Html Msg
paragraph { identifier, lines } =
    lines
        |> List.map Html.text
        |> List.intersperse (Html.br [] [])
        |> Html.p
            [ id identifier
            , style Styles.paragraph
            ]


button : Model -> Html Msg
button { tokenTransfer, authToken, buttonState } =
    Html.span
        [ id "copy-token"
        , style <| Styles.button buttonState
        , onMouseEnter <| CopyTokenButtonHover True
        , onMouseLeave <| CopyTokenButtonHover False
        , onClick CopyToken
        , attribute "data-clipboard-text" authToken
        ]
        [ Html.div
            [ id "copy-icon"
            , style Styles.buttonIcon
            ]
            []
        , Html.text <| Text.button buttonState
        ]
