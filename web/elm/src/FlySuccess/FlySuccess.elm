module FlySuccess.FlySuccess exposing
    ( handleCallback
    , init
    , update
    , view
    )

import Callback exposing (Callback(..))
import Effects exposing (Effect(..))
import FlySuccess.Models
    exposing
        ( ButtonState(..)
        , Model
        , TokenTransfer
        , TransferResult
        , hover
        , isClicked
        , isPending
        )
import FlySuccess.Msgs exposing (Msg(..))
import FlySuccess.Styles as Styles
import FlySuccess.Text as Text
import Html exposing (Html)
import Html.Attributes exposing (attribute, class, id, style)
import Html.Events exposing (onClick, onMouseEnter, onMouseLeave)
import Html.Styled as HS
import Routes
import TopBar.Model
import TopBar.Styles
import TopBar.TopBar as TopBar
import UserState exposing (UserState)


init : { authToken : String, flyPort : Maybe Int } -> ( Model, List Effect )
init { authToken, flyPort } =
    let
        ( topBar, topBarEffects ) =
            TopBar.init { route = Routes.FlySuccess { flyPort = flyPort } }
    in
    ( { buttonState = Unhovered
      , authToken = authToken
      , tokenTransfer =
            case flyPort of
                Just _ ->
                    Nothing

                Nothing ->
                    Just <| Err ()
      , isUserMenuExpanded = topBar.isUserMenuExpanded
      , isPinMenuExpanded = topBar.isPinMenuExpanded
      , route = topBar.route
      , groups = topBar.groups
      , dropdown = topBar.dropdown
      , screenSize = topBar.screenSize
      , shiftDown = topBar.shiftDown
      }
    , topBarEffects
        ++ (case flyPort of
                Just fp ->
                    [ SendTokenToFly authToken fp ]

                Nothing ->
                    []
           )
    )


handleCallback : Callback -> ( Model, List Effect ) -> ( Model, List Effect )
handleCallback msg ( model, effects ) =
    case msg of
        TokenSentToFly success ->
            ( { model | tokenTransfer = Just <| Ok success }, effects )

        _ ->
            TopBar.handleCallback msg ( model, effects )


update : Msg -> ( Model, List Effect ) -> ( Model, List Effect )
update msg ( model, effects ) =
    case msg of
        CopyTokenButtonHover hovered ->
            ( { model | buttonState = hover hovered model.buttonState }
            , effects
            )

        CopyToken ->
            ( { model | buttonState = Clicked }, effects )

        FromTopBar msg ->
            TopBar.update msg ( model, effects )


view : UserState -> Model -> Html Msg
view userState model =
    Html.div []
        [ Html.div
            [ style TopBar.Styles.pageIncludingTopBar
            , id "page-including-top-bar"
            ]
            [ TopBar.view userState TopBar.Model.None model |> HS.toUnstyled |> Html.map FromTopBar
            , Html.div [ id "page-below-top-bar", style TopBar.Styles.pageBelowTopBar ]
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
