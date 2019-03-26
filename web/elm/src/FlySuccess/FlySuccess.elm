module FlySuccess.FlySuccess exposing
    ( handleCallback
    , init
    , update
    , view
    )

import EffectTransformer exposing (ET)
import FlySuccess.Models
    exposing
        ( ButtonState(..)
        , Model
        , TokenTransfer
        , TransferFailure(..)
        , hover
        , isClicked
        , isPending
        )
import FlySuccess.Styles as Styles
import FlySuccess.Text as Text
import Html exposing (Html)
import Html.Attributes exposing (attribute, class, id, style)
import Html.Events exposing (onClick, onMouseEnter, onMouseLeave)
import Login.Login as Login
import Message.Callback exposing (Callback(..))
import Message.Effects exposing (Effect(..))
import Message.Message exposing (Hoverable(..), Message(..))
import RemoteData
import Routes
import UserState exposing (UserState)
import Views.Icon as Icon
import Views.Styles
import Views.TopBar as TopBar


init : { authToken : String, flyPort : Maybe Int } -> ( Model, List Effect )
init { authToken, flyPort } =
    ( { buttonState = Unhovered
      , authToken = authToken
      , tokenTransfer =
            case flyPort of
                Just _ ->
                    RemoteData.Loading

                Nothing ->
                    RemoteData.Failure NoFlyPort
      , isUserMenuExpanded = False
      }
    , case flyPort of
        Just fp ->
            [ SendTokenToFly authToken fp ]

        Nothing ->
            []
    )


handleCallback : Callback -> ET Model
handleCallback msg ( model, effects ) =
    case msg of
        TokenSentToFly (Ok ()) ->
            ( { model | tokenTransfer = RemoteData.Success () }, effects )

        TokenSentToFly (Err err) ->
            ( { model | tokenTransfer = RemoteData.Failure (NetworkTrouble err) }, effects )

        _ ->
            ( model, effects )


update : Message -> ET Model
update msg ( model, effects ) =
    case msg of
        Hover (Just CopyTokenButton) ->
            ( { model | buttonState = hover True model.buttonState }
            , effects
            )

        Hover Nothing ->
            ( { model | buttonState = hover False model.buttonState }
            , effects
            )

        CopyToken ->
            ( { model | buttonState = Clicked }, effects )

        _ ->
            ( model, effects )


view : UserState -> Model -> Html Message
view userState model =
    Html.div []
        [ Html.div
            [ style Views.Styles.pageIncludingTopBar
            , id "page-including-top-bar"
            ]
            [ Html.div
                [ id "top-bar-app"
                , style <| Views.Styles.topBar False
                ]
                [ TopBar.concourseLogo
                , Login.view userState model False
                ]
            , Html.div
                [ id "page-below-top-bar"
                , style <| Views.Styles.pageBelowTopBar <| Routes.FlySuccess { flyPort = Nothing }
                ]
                [ Html.div
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
                ]
            ]
        ]


body : Model -> List (Html Message)
body model =
    let
        elemList =
            List.filter Tuple.second >> List.map Tuple.first
    in
    case model.tokenTransfer of
        RemoteData.Loading ->
            [ Html.text Text.pending ]

        RemoteData.NotAsked ->
            [ Html.text Text.pending ]

        RemoteData.Success () ->
            elemList
                [ ( paragraph
                        { identifier = "first-paragraph"
                        , lines = Text.firstParagraphSuccess
                        }
                  , True
                  )
                , ( button model, False )
                , ( paragraph
                        { identifier = "second-paragraph"
                        , lines = Text.secondParagraphSuccess
                        }
                  , True
                  )
                ]

        RemoteData.Failure err ->
            elemList
                [ ( paragraph
                        { identifier = "first-paragraph"
                        , lines = Text.firstParagraphFailure
                        }
                  , True
                  )
                , ( button model, True )
                , ( paragraph
                        { identifier = "second-paragraph"
                        , lines = Text.secondParagraphFailure err
                        }
                  , True
                  )
                ]


paragraph : { identifier : String, lines : Text.Paragraph } -> Html Message
paragraph { identifier, lines } =
    lines
        |> List.map Html.text
        |> List.intersperse (Html.br [] [])
        |> Html.p
            [ id identifier
            , style Styles.paragraph
            ]


button : Model -> Html Message
button { tokenTransfer, authToken, buttonState } =
    Html.span
        [ id "copy-token"
        , style <| Styles.button buttonState
        , onMouseEnter <| Hover <| Just CopyTokenButton
        , onMouseLeave <| Hover Nothing
        , onClick CopyToken
        , attribute "data-clipboard-text" authToken
        ]
        [ Icon.icon
            { sizePx = 20
            , image = "clippy.svg"
            }
            [ id "copy-icon"
            , style [ ( "margin-right", "5px" ) ]
            ]
        , Html.text <| Text.button buttonState
        ]
