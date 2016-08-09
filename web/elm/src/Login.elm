module Login exposing (..)

import Erl
import Html exposing (Html)
import Html.Attributes as Attributes exposing (id, class)
import Html.Events as Events
import Http
import Json.Decode exposing ((:=))
import Navigation
import String
import Task

import Concourse.AuthMethod exposing (AuthMethod (..))
import Concourse.Team exposing (Team)

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
  , teams : Maybe (List Team)
  , redirect : String
  }

type alias LoginModel =
  { teamName : String
  , authMethods : Maybe (List AuthMethod)
  , hasTeamSelectionInBrowserHistory : Bool
  , redirect : String
  }

type Action
  = Noop
  | FilterTeams String
  | TeamsFetched (Result Http.Error (List Team))
  | SelectTeam String
  | AuthFetched (Result Http.Error (List AuthMethod))
  | GoBack

defaultPage : PageWithRedirect
defaultPage = { page = TeamSelectionPage, redirect = "" }

init : Result String PageWithRedirect -> (Model, Cmd Action)
init pageResult =
  let
    pageWithRedirect = Result.withDefault defaultPage pageResult
  in
    case pageWithRedirect.page of
      TeamSelectionPage ->
        ( TeamSelection
            { teamFilter = ""
            , teams = Nothing
            , redirect = pageWithRedirect.redirect
            }
        , Cmd.map TeamsFetched <| Task.perform Err Ok Concourse.Team.fetchTeams
        )
      LoginPage teamName ->
        ( Login
            { teamName = teamName
            , authMethods = Nothing
            , hasTeamSelectionInBrowserHistory = False
            , redirect = pageWithRedirect.redirect
            }
        , Cmd.map
            AuthFetched <|
            Task.perform
              Err Ok <|
                Concourse.AuthMethod.fetchAuthMethods teamName
        )

urlUpdate : Result String PageWithRedirect -> Model -> (Model, Cmd Action)
urlUpdate pageResult model =
  let
    pageWithRedirect = Result.withDefault defaultPage pageResult
  in
    case pageWithRedirect.page of
      TeamSelectionPage ->
        ( TeamSelection
            { teamFilter = ""
            , teams = Nothing
            , redirect = pageWithRedirect.redirect
            }
        , Cmd.map TeamsFetched <| Task.perform Err Ok Concourse.Team.fetchTeams
        )
      LoginPage teamName ->
        ( Login
            { teamName = teamName
            , authMethods = Nothing
            , hasTeamSelectionInBrowserHistory = True
            , redirect = pageWithRedirect.redirect
            }
        , Cmd.map
            AuthFetched <|
            Task.perform
              Err Ok <|
                Concourse.AuthMethod.fetchAuthMethods teamName
        )

update : Action -> Model -> (Model, Cmd Action)
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

view : Model -> Html Action
view model =
  case model of
    TeamSelection tModel -> viewTeamSelection tModel
    Login lModel -> viewLogin lModel

viewLoading : Html action
viewLoading =
  Html.div [class "loading"]
    [ Html.i [class "fa fa-fw fa-spin fa-circle-o-notch"] []
    , Html.text "Loading..."
    ]

viewTeamSelection : TeamSelectionModel -> Html Action
viewTeamSelection model =
  let filteredTeams =
    filterTeams model.teamFilter <| Maybe.withDefault [] model.teams
  in
    Html.div
      [ class "centered-contents" ]
      [ Html.div
          [ class "small-title" ]
          [ Html.text "select a team to login" ]
      , Html.div
          [ class "login-box team-selection" ]
          [ Html.form
              [ Events.onSubmit <|
                  case (List.head filteredTeams, model.teamFilter) of
                    (Nothing, _) -> Noop
                    (Just _, "") -> Noop
                    (Just firstTeam, _) -> SelectTeam firstTeam.name
              , class "filter-form input-holder"
              ]
              [ Html.i [class "fa fa-fw fa-search"] []
              , Html.input
                  [ Attributes.placeholder "filter teams"
                  , Attributes.autofocus True
                  , Events.onInput FilterTeams
                  ]
                  []
              ]
          , case model.teams of
              Nothing -> viewLoading
              Just _ ->
                Html.div
                  [] <|
                  List.map (viewTeam model.redirect) filteredTeams
          ]
      ]

onClickPreventDefault : msg -> Html.Attribute msg
onClickPreventDefault message =
  Events.onWithOptions
    "click"
    {stopPropagation = False, preventDefault = True} <|
    Json.Decode.customDecoder
      ("button" := Json.Decode.int) <|
      assertLeftButton message

assertLeftButton : a -> Int -> Result String a
assertLeftButton message button =
  if button == 0 then Ok message
  else Err "placeholder error, nothing is wrong"

viewTeam : String -> Team -> Html Action
viewTeam redirect team =
  Html.a
    [ onClickPreventDefault <| SelectTeam team.name
    , Attributes.href <| loginRoute redirect team.name
    ]
    [ Html.text <| team.name ]

filterTeams : String -> List Team -> List Team
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

teamNameContains : String -> Team -> Bool
teamNameContains substring team =
  String.contains substring <|
    String.toLower team.name

teamNameStartsWith : String -> Team -> Bool
teamNameStartsWith substring team =
  String.startsWith substring <|
    String.toLower team.name

teamNameStartsWithSensitive : String -> Team -> Bool
teamNameStartsWithSensitive substring team =
  String.startsWith substring team.name

viewLogin : LoginModel -> Html Action
viewLogin model =
  Html.div
    [ class "centered-contents" ]
    [ Html.div
        [ class "small-title" ]
        [ Html.a
            [ onClickPreventDefault GoBack
            , Attributes.href <| teamSelectionRoute model.redirect
            ]
            [ Html.i [class "fa fa-fw fa-chevron-left"] []
            , Html.text "back to team selection"
            ]
        ]
    , Html.div
        [ class "login-box auth-methods" ] <|
        [ Html.div
            [ class "centered-contents auth-methods-title" ]
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
    [ class "padded-top centered-contents"
    , Attributes.method "post"
    ]
    [ Html.button
        [ Attributes.type' "submit" ]
        [ Html.text "login" ]
    ]

viewBasicAuthForm : List AuthMethod -> Maybe (Html action)
viewBasicAuthForm methods =
  if List.member BasicMethod methods then
    Just <|
      Html.form
        [ class "padded-top"
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
        , Html.div
            [ class "input-holder" ]
            [ Html.input
                [ id "basic-auth-password-input"
                , Attributes.name "password"
                , Attributes.type' "password"
                ]
                []
            ]
        , Html.div
            [ class "centered-contents" ]
            [ Html.button
                [ Attributes.type' "submit" ]
                [ Html.text "login" ]
            ]
        ]
  else Nothing

viewOAuthButtons : String -> List AuthMethod -> Maybe (Html action)
viewOAuthButtons redirect methods =
  case List.filterMap (viewOAuthButton redirect) methods of
    [] -> Nothing
    buttons ->
      Just <|
        Html.div [ class "centered-contents padded-top" ] buttons

viewOAuthButton : String -> AuthMethod -> Maybe (Html action)
viewOAuthButton redirect method =
  case method of
    BasicMethod -> Nothing
    OAuthMethod oAuthMethod ->
      Just <|
        Html.a
          [ Attributes.href <| routeWithRedirect redirect oAuthMethod.authUrl ]
          [ Html.text <| "login with " ++ oAuthMethod.displayName ]
