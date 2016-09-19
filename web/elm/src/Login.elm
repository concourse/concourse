module Login exposing (Model, Msg, Page(..), PageWithRedirect, init, update, urlUpdate, view, subscriptions)

import Erl
import Html exposing (Html)
import Html.Attributes as Attributes exposing (id, class)
import Html.Events as Events
import Http
import Navigation
import String
import Task

import Concourse
import Concourse.AuthMethod
import Concourse.Team
import Routes
import StrictEvents exposing (onLeftClick)

type alias PageWithRedirect =
  { page : Page
  , redirect : String
  }

type Page = TeamSelectionPage | LoginPage String

type Model
  = TeamSelection TeamSelectionModel
  | Login LoginModel

type alias TeamSelectionModel =
  { teamFilter : String
  , teams : Maybe (List Concourse.Team)
  , redirect : String
  }

type alias LoginModel =
  { teamName : String
  , authMethods : Maybe (List Concourse.AuthMethod)
  , hasTeamSelectionInBrowserHistory : Bool
  , redirect : String
  }

type Msg
  = Noop
  | FilterTeams String
  | TeamsFetched (Result Http.Error (List Concourse.Team))
  | SelectTeam String
  | AuthFetched (Result Http.Error (List Concourse.AuthMethod))
  | GoBack

defaultPage : PageWithRedirect
defaultPage = { page = TeamSelectionPage, redirect = "" }

init : Routes.Route -> (Model, Cmd Msg)
init route =
  case route of
    Routes.TeamLogin teamName ->
      ( Login
          { teamName = teamName
          , authMethods = Nothing
          , hasTeamSelectionInBrowserHistory = False
          , redirect = ""
          }
      , Cmd.map
          AuthFetched <|
          Task.perform
            Err Ok <|
              Concourse.AuthMethod.fetchAll teamName
      )
    _ ->
      ( TeamSelection
          { teamFilter = ""
          , teams = Nothing
          , redirect = ""
          }
      , Cmd.map TeamsFetched <| Task.perform Err Ok Concourse.Team.fetchTeams
      )

urlUpdate : Routes.Route -> Model -> (Model, Cmd Msg)
urlUpdate route model =
  case route of
    Routes.TeamLogin teamName ->
      ( Login
          { teamName = teamName
          , authMethods = Nothing
          , hasTeamSelectionInBrowserHistory = True
          , redirect = ""
          }
      , Cmd.map
          AuthFetched <|
          Task.perform
            Err Ok <|
              Concourse.AuthMethod.fetchAll teamName
      )
    _ ->
      ( TeamSelection
          { teamFilter = ""
          , teams = Nothing
          , redirect = ""
          }
      , Cmd.map TeamsFetched <| Task.perform Err Ok Concourse.Team.fetchTeams
      )


update : Msg -> Model -> (Model, Cmd Msg)
update action model =
  case action of
    Noop ->
      (model, Cmd.none)
    FilterTeams newTeamFilter ->
      case model of
        TeamSelection teamSelectionModel ->
          ( TeamSelection { teamSelectionModel | teamFilter = newTeamFilter }
          , Cmd.none
          )
        Login _ -> (model, Cmd.none)
    TeamsFetched (Ok teams) ->
      case model of
        TeamSelection teamSelectionModel ->
          ( TeamSelection { teamSelectionModel | teams = Just teams }
          , Cmd.none
          )
        Login _ -> (model, Cmd.none)
    TeamsFetched (Err err) ->
      Debug.log ("failed to fetch teams: " ++ toString err) <|
        (model, Cmd.none)
    SelectTeam teamName ->
      case model of
        TeamSelection tsModel ->
          ( model
          , Navigation.newUrl <| loginRoute tsModel.redirect teamName
          )
        Login _ -> (model, Cmd.none)
    AuthFetched (Ok authMethods) ->
      case model of
        Login loginModel ->
          ( Login { loginModel | authMethods = Just authMethods }
          , Cmd.none
          )
        TeamSelection tModel ->
          (model, Cmd.none)
    AuthFetched (Err err) ->
      Debug.log ("failed to fetch auth methods: " ++ toString err) <|
        (model, Cmd.none)
    GoBack ->
      case model of
        Login loginModel ->
          case loginModel.hasTeamSelectionInBrowserHistory of
            True -> (model, Navigation.back 1)
            False -> (model, Navigation.newUrl <| teamSelectionRoute loginModel.redirect)
        TeamSelection _ -> (model, Cmd.none)

loginRoute : String -> String -> String
loginRoute redirect teamName =
  routeMaybeRedirect redirect <| "teams/" ++ teamName ++ "/login"

teamSelectionRoute : String -> String
teamSelectionRoute redirect = routeMaybeRedirect redirect "/login"

routeMaybeRedirect : String -> String -> String
routeMaybeRedirect redirect route =
  if redirect /= "" then
    let
      parsedRoute = Erl.parse route
    in let
      newParsedRoute = Erl.addQuery "redirect" redirect parsedRoute
    in
      Erl.toString newParsedRoute
  else route

routeWithRedirect : String -> String -> String
routeWithRedirect redirect route =
  let
    parsedRoute = Erl.parse route
    actualRedirect =
      case redirect of
        "" -> indexPageUrl
        _ -> redirect
  in let
    newParsedRoute = Erl.addQuery "redirect" actualRedirect parsedRoute
  in
    Erl.toString newParsedRoute

indexPageUrl : String
indexPageUrl = "/"

view : Model -> Html Msg
view model =
  Html.div [class "login-page"] [
    case model of
      TeamSelection tModel ->
        viewTeamSelection tModel

      Login lModel ->
        viewLogin lModel
  ]

viewLoading : Html action
viewLoading =
  Html.div [class "loading"]
    [ Html.i [class "fa fa-fw fa-spin fa-circle-o-notch"] []
    ]

viewTeamSelection : TeamSelectionModel -> Html Msg
viewTeamSelection model =
  let filteredTeams =
    filterTeams model.teamFilter <| Maybe.withDefault [] model.teams
  in
    Html.div
      []
      [ Html.div
          [ class "small-title" ]
          [ Html.text "select a team to login" ]
      , Html.div
          [ class "login-box team-selection" ]
          [ Html.form
              [ Events.onSubmit <|
                  case (List.head filteredTeams, model.teamFilter) of
                    (Nothing, _) ->
                      Noop
                    (Just _, "") ->
                      Noop
                    (Just firstTeam, _) ->
                      SelectTeam firstTeam.name
              , class "filter-form input-holder"
              ]
              [ Html.i [class "fa fa-fw fa-search search-icon"] []
              , Html.input
                  [ class "search-input"
                  , Attributes.placeholder "filter teams"
                  , Attributes.autofocus True
                  , Attributes.required True
                  , Events.onInput FilterTeams
                  ]
                  []
              , Html.button
                  [ class "clear-button"
                  , Attributes.type' "reset"
                  , Attributes.tabindex -1
                  , Events.onClick (FilterTeams "")
                  ]
                  [ Html.i [class "fa fa-fw fa-times-circle"] [] ]
              ]
          , case model.teams of
              Nothing ->
                viewLoading
              Just _ ->
                Html.div [class "teams-list"] <|
                  List.map (viewTeam model.redirect) filteredTeams
          ]
      ]

viewTeam : String -> Concourse.Team -> Html Msg
viewTeam redirect team =
  Html.a
    [ onLeftClick (SelectTeam team.name)
    , Attributes.href <| loginRoute redirect team.name
    ]
    [ Html.text <| team.name ]

filterTeams : String -> List Concourse.Team -> List Concourse.Team
filterTeams teamFilter teams =
  let
    filteredList =
      List.filter
        (teamNameContains <| String.toLower teamFilter) teams
  in let
    (startingTeams, notStartingTeams) =
      List.partition (teamNameStartsWith <| String.toLower teamFilter) filteredList
  in let
    (caseSensitive, notCaseSensitive) =
      List.partition (teamNameStartsWithSensitive teamFilter) startingTeams
  in
    caseSensitive ++ notCaseSensitive ++ notStartingTeams

teamNameContains : String -> Concourse.Team -> Bool
teamNameContains substring team =
  String.contains substring <|
    String.toLower team.name

teamNameStartsWith : String -> Concourse.Team -> Bool
teamNameStartsWith substring team =
  String.startsWith substring <|
    String.toLower team.name

teamNameStartsWithSensitive : String -> Concourse.Team -> Bool
teamNameStartsWithSensitive substring team =
  String.startsWith substring team.name

viewLogin : LoginModel -> Html Msg
viewLogin model =
  Html.div []
    [ Html.div
        [ class "small-title" ]
        [ Html.a
            [ onLeftClick GoBack
            , Attributes.href <| teamSelectionRoute model.redirect
            ]
            [ Html.i [class "fa fa-fw fa-chevron-left"] []
            , Html.text "back to team selection"
            ]
        ]
    , Html.div
        [ class "login-box auth-methods" ] <|
        [ Html.div
            [ class "auth-methods-title" ]
            [ Html.text "logging in to "
            , Html.span
                [ class "bright-text" ]
                [ Html.text model.teamName ]
            ]
        ] ++
          case model.authMethods of
            Nothing -> [ viewLoading ]
            Just methods ->
              case (viewBasicAuthForm methods, viewOAuthButtons model.redirect methods) of
                (Just basicForm, Just buttons) ->
                  [ buttons, viewOrBar, basicForm ]
                (Just basicForm, Nothing) -> [ basicForm ]
                (Nothing, Just buttons) -> [ buttons ]
                (Nothing, Nothing) -> [ viewNoAuthButton ]
    ]

viewOrBar : Html action
viewOrBar =
  Html.div
    [ class "or-bar" ]
    [ Html.div [] []
    , Html.span [] [ Html.text "or" ]
    ]

viewNoAuthButton : Html action
viewNoAuthButton =
  Html.form
    [ class "auth-method login-button"
    , Attributes.method "post"
    ]
    [ Html.button
        [ Attributes.type' "submit" ]
        [ Html.text "login" ]
    ]

viewBasicAuthForm : List Concourse.AuthMethod -> Maybe (Html action)
viewBasicAuthForm methods =
  if List.member Concourse.AuthMethodBasic methods then
    Just <|
      Html.form
        [ class "auth-method basic-auth"
        , Attributes.method "post"
        ]
        [ Html.label
            [ Attributes.for "basic-auth-username-input" ]
            [ Html.text "username" ]
        , Html.div
            [ class "input-holder" ]
            [ Html.input
                [ id "basic-auth-username-input"
                , Attributes.name "username"
                , Attributes.type' "text"
                ]
                []
            ]
        , Html.label
            [ Attributes.for "basic-auth-password-input" ]
            [ Html.text "password" ]
        , Html.div [class "input-holder"] -- for LastPass web UI
            [ Html.input
                [ id "basic-auth-password-input"
                , Attributes.name "password"
                , Attributes.type' "password"
                ]
                []
            ]
        , Html.div
            [ class "login-button" ]
            [ Html.button
                [ Attributes.type' "submit" ]
                [ Html.text "login" ]
            ]
        ]
  else
    Nothing

viewOAuthButtons : String -> List Concourse.AuthMethod -> Maybe (Html action)
viewOAuthButtons redirect methods =
  case List.filterMap (viewOAuthButton redirect) methods of
    [] ->
      Nothing

    buttons ->
      Just <|
        Html.div [class "oauth-buttons"] buttons

viewOAuthButton : String -> Concourse.AuthMethod -> Maybe (Html action)
viewOAuthButton redirect method =
  case method of
    Concourse.AuthMethodBasic ->
      Nothing
    Concourse.AuthMethodOAuth oAuthMethod ->
      Just <|
        Html.div [class "auth-method login-button"] [
          Html.a
            [ Attributes.href <| routeWithRedirect redirect oAuthMethod.authUrl ]
            [ Html.text <| "login with " ++ oAuthMethod.displayName ]
        ]

subscriptions : Model -> Sub Msg
subscriptions model =
  Sub.none
