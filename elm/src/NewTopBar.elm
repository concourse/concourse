module NewTopBar exposing (Model, Msg, init, update, view)

import Concourse
import Concourse.User
import Html exposing (Html)
import Html.Attributes exposing (class, href, src, type_, placeholder)
import RemoteData exposing (RemoteData)


type alias Model =
    { user : RemoteData.WebData Concourse.User }


type Msg
    = UserFetched (RemoteData.WebData Concourse.User)


init : ( Model, Cmd Msg )
init =
    ( { user = RemoteData.Loading }, fetchUser )


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        UserFetched response ->
            ( { model | user = response }, Cmd.none )


showUserInfo : Model -> Html Msg
showUserInfo model =
    case model.user of
        RemoteData.NotAsked ->
            Html.text "n/a"

        RemoteData.Loading ->
            Html.text "loading"

        RemoteData.Success user ->
            Html.text user.team.name

        RemoteData.Failure _ ->
            Html.text "log in"


view : Model -> Html Msg
view model =
    Html.div [ class "module-topbar" ]
        [ Html.div [ class "topbar-logo" ] [ Html.a [ class "logo-image-link", href "#" ] [] ]
        , Html.div [ class "topbar-search" ]
            [ Html.form [ class "topbar-search-form" ]
                [ Html.input [ class "search-input-field", type_ "text", placeholder "search" ] []
                , Html.button [ class "search-clear-button" ] []
                ]
            ]
        , Html.div [ class "topbar-login" ]
            [ Html.div [ class "topbar-user-info" ]
                [ showUserInfo model ]
            ]
        ]


fetchUser : Cmd Msg
fetchUser =
    Cmd.map UserFetched <|
        RemoteData.asCmd Concourse.User.fetchUser
