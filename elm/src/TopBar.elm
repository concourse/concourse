port module TopBar exposing (Model, Msg(..), init, update, urlUpdate, view, subscriptions, fetchUser)

import Html exposing (Html)
import Html.Attributes exposing (class, classList, href, id, disabled, attribute, style)
import Html.Events exposing (onClick)
import Http
import List
import Navigation exposing (Location)
import Route.QueryString as QueryString
import String
import Task
import Time

import Concourse
import Concourse.Pipeline
import Concourse.User
import Routes
import StrictEvents exposing (onLeftClickOrShiftLeftClick)

type alias Model =
  { pipelineIdentifier : Maybe Concourse.PipelineIdentifier
  , route : Routes.ConcourseRoute
  , selectedGroups : List String
  , pipeline : Maybe Concourse.Pipeline
  , userState : UserState
  , userMenuVisible : Bool
  }

type UserState
  = UserStateLoggedIn Concourse.User
  | UserStateLoggedOut
  | UserStateUnknown

type Msg
  = Noop
  | PipelineFetched (Result Http.Error Concourse.Pipeline)
  | UserFetched (Result Concourse.User.Error Concourse.User)
  | FetchPipeline Concourse.PipelineIdentifier
  | ToggleSidebar
  | ToggleGroup Concourse.PipelineGroup
  | SetGroups (List String)
  | LogOut
  | LogIn
  | NavTo String
  | LoggedOut (Result Concourse.User.Error ())
  | ToggleUserMenu

queryGroupsForRoute : Routes.ConcourseRoute -> List String
queryGroupsForRoute route =
  QueryString.all "groups" route.queries

init : Routes.ConcourseRoute -> (Model, Cmd Msg)
init route =
  let
    (model, cmd) =
      updateModel
        route
        { pipelineIdentifier = Nothing
        , selectedGroups = queryGroupsForRoute route
        , route = route
        , pipeline = Nothing
        , userState = UserStateUnknown
        , userMenuVisible = False
        }
  in
    (model, Cmd.batch[cmd, fetchUser])

updateModel : Routes.ConcourseRoute -> Model -> (Model, Cmd Msg)
updateModel route model =
  let
    pid =
      extractPidFromRoute route.logical
  in
    ( { model | pipelineIdentifier = pid }
    , case pid of
        Nothing ->
          Cmd.none
        Just pid ->
          fetchPipeline pid
    )

update : Msg -> Model -> (Model, Cmd Msg)
update msg model =
  case msg of
    Noop ->
      (model, Cmd.none)

    FetchPipeline pid ->
      (model, fetchPipeline pid)

    UserFetched (Ok user) ->
      ( { model | userState = UserStateLoggedIn user }
      , Cmd.none
      )

    UserFetched (Err _) ->
      ( { model | userState = UserStateLoggedOut }
      , Cmd.none
      )

    PipelineFetched (Ok pipeline) ->
      case model.pipeline of
        Nothing ->
          ( { model
            | pipeline = Just pipeline
            , pipelineIdentifier = Just { teamName = pipeline.teamName, pipelineName = pipeline.name }
            }
          , Cmd.none
          )
        Just p ->
          if List.isEmpty pipeline.groups then
              ( { model
                | pipeline = Just pipeline
                , pipelineIdentifier = Just { teamName = pipeline.teamName, pipelineName = pipeline.name }
                }
              , Cmd.none
              )
            else
              if pipeline.groups == p.groups then
                (model, Cmd.none)
              else
                ( { model
                  | pipeline = Just pipeline
                  , pipelineIdentifier = Just { teamName = pipeline.teamName, pipelineName = pipeline.name }
                  }
                , Cmd.none
                )

    PipelineFetched (Err err) ->
      Debug.log
        ("failed to load pipeline: " ++ toString err)
        (model, Cmd.none)

    ToggleSidebar ->
      Debug.log "sidebar-toggle-incorrectly-handled: " (model, Cmd.none)

    ToggleGroup group ->
      setGroups (toggleGroup group model.selectedGroups model.pipeline) model

    SetGroups groups ->
      setGroups groups model

    LogIn ->
      ( { model
        | pipelineIdentifier = Nothing
        , selectedGroups = []
        , pipeline = Nothing
        }
      , Navigation.newUrl "/login"
      )

    LogOut ->
      (model, logOut)

    LoggedOut (Ok _) ->
      ( { model
        | userState = UserStateLoggedOut
        , pipelineIdentifier = Nothing
        , pipeline = Nothing
        , selectedGroups = []
        }
      , Navigation.newUrl "/"
      )

    NavTo url ->
      (model, Navigation.newUrl url)

    LoggedOut (Err msg) ->
      always (model, Cmd.none) <|
        Debug.log "failed to log out" msg

    ToggleUserMenu ->
      ({ model | userMenuVisible = not model.userMenuVisible }, Cmd.none)

subscriptions : Model -> Sub Msg
subscriptions model =
  case model.pipelineIdentifier of
    Nothing ->
      Sub.none
    Just pid ->
      Time.every (5 * Time.second) (always (FetchPipeline pid))

extractPidFromRoute : Routes.Route -> Maybe Concourse.PipelineIdentifier
extractPidFromRoute route =
  case route of
    Routes.Build teamName pipelineName jobName buildName ->
      Just {teamName = teamName, pipelineName = pipelineName}
    Routes.Job teamName pipelineName jobName ->
      Just {teamName = teamName, pipelineName = pipelineName}
    Routes.Resource teamName pipelineName resourceName ->
      Just {teamName = teamName, pipelineName = pipelineName}
    Routes.OneOffBuild buildId ->
      Nothing
    Routes.Pipeline teamName pipelineName ->
      Just {teamName = teamName, pipelineName = pipelineName}
    Routes.SelectTeam ->
      Nothing
    Routes.TeamLogin teamName ->
      Nothing
    Routes.Home ->
      Nothing


setGroups : List String -> Model -> (Model, Cmd Msg)
setGroups newGroups model =
  let
    newUrl =
      pidToUrl model.pipelineIdentifier <|
        setGroupsInLocation model.route newGroups
  in
    (model, Navigation.newUrl newUrl)

urlUpdate : Routes.ConcourseRoute -> Model -> (Model, Cmd Msg)
urlUpdate route model =
  let
    pipelineIdentifier =
      extractPidFromRoute route.logical
  in
    ( { model
      | pipelineIdentifier = model.pipelineIdentifier
      , route = route
      , selectedGroups = queryGroupsForRoute route
      }
    , case pipelineIdentifier of
        Nothing ->
          Cmd.none
        Just pid ->
          fetchPipeline pid
    )

getDefaultSelectedGroups : Maybe Concourse.Pipeline -> List String
getDefaultSelectedGroups pipeline =
  case pipeline of
    Nothing ->
      []
    Just pipeline ->
      case List.head pipeline.groups of
        Nothing ->
          []
        Just first ->
          [first.name]


setGroupsInLocation : Routes.ConcourseRoute -> List String -> Routes.ConcourseRoute
setGroupsInLocation loc groups =
  let
    updatedUrl =
      if List.isEmpty groups then
        QueryString.remove "groups" loc.queries
      else
        List.foldr
          (QueryString.add "groups") QueryString.empty groups
  in
    { loc
    | queries = updatedUrl
    }

pidToUrl : Maybe Concourse.PipelineIdentifier -> Routes.ConcourseRoute -> String
pidToUrl pid {queries} =
  case pid of
    Just {teamName, pipelineName} ->
      String.join ""
        [ String.join "/"
            [ "/teams", teamName, "pipelines", pipelineName
            ]
        , QueryString.render queries
        ]
    Nothing ->
      ""

toggleGroup : Concourse.PipelineGroup -> List String -> Maybe Concourse.Pipeline -> List String
toggleGroup grp names mpipeline =
  if List.member grp.name names then
    List.filter ((/=) grp.name) names
  else
    if List.isEmpty names then
      grp.name :: (getDefaultSelectedGroups mpipeline)
    else
      grp.name :: names

getSelectedOrDefaultGroups : Model -> List String
getSelectedOrDefaultGroups model =
  if List.isEmpty model.selectedGroups then
    getDefaultSelectedGroups model.pipeline
  else
    model.selectedGroups

view : Model -> Html Msg
view model =
  Html.nav
    [ classList
        [ ("top-bar", True)
        , ("test", True)
        , ("paused", isPaused model.pipeline)
        ]
    ]
    [ let
        groupList =
          case model.pipeline of
            Nothing ->
              []
            Just pipeline ->
              List.map
                (viewGroup (getSelectedOrDefaultGroups model) pipeline.url)
                pipeline.groups
      in
        Html.ul [class "groups"] <|
          [ Html.li [class "main"]
              [ Html.span
                  [ class "sidebar-toggle test btn-hamburger"
                  , onClick ToggleSidebar
                  , Html.Attributes.attribute "aria-label" "Toggle List of Pipelines"
                  ]
                  [ Html.i [class "fa fa-bars"] []
                  ]
              ]
           , Html.li [class "main"]
              [ Html.a
                  [ StrictEvents.onLeftClick <| NavTo "/"
                  , Html.Attributes.href <|
                      Maybe.withDefault "/" (Maybe.map .url model.pipeline)
                  ]
                  [ Html.i [class "fa fa-home"] []
                  ]
              ]
          ] ++ groupList
    , Html.ul [class "nav-right"]
        [ Html.li [class "nav-item"]
            [ viewUserState model.userState model.userMenuVisible
            ]
        ]
    ]

isPaused : Maybe Concourse.Pipeline -> Bool
isPaused =
  Maybe.withDefault False << Maybe.map .paused

viewUserState : UserState -> Bool -> Html Msg
viewUserState userState userMenuVisible =
  case userState of
    UserStateUnknown ->
      Html.text ""

    UserStateLoggedOut ->
      Html.div [class "user-info"]
        [ Html.a
            [ StrictEvents.onLeftClick <| LogIn
            , href "/login"
            , Html.Attributes.attribute "aria-label" "Log In"
            , class "login-button"
            ]
            [ Html.text "login"
            ]
        ]

    UserStateLoggedIn {team} ->
      Html.div [class "user-info"]
        [ Html.div [class "user-id", onClick ToggleUserMenu]
            [ Html.i [class "fa fa-user"] []
            , Html.text " "
            , Html.text team.name
            , Html.text " "
            , Html.i [class "fa fa-caret-down"] []
            ]
        , Html.div [classList [("user-menu", True), ("hidden", not userMenuVisible)]]
            [ Html.a
                [ Html.Attributes.attribute "aria-label" "Log Out"
                , onClick LogOut
                ]
                [ Html.text "logout"
                ]
            ]
        ]

viewGroup : List String -> String -> Concourse.PipelineGroup -> Html Msg
viewGroup selectedGroups url grp =
  Html.li
    [ if List.member grp.name selectedGroups
        then class "main active"
        else class "main"
    ]
    [ Html.a
        [ Html.Attributes.href <| url ++ "?groups=" ++ grp.name
        , onLeftClickOrShiftLeftClick (SetGroups [grp.name]) (ToggleGroup grp)
        ]
        [ Html.text grp.name]
    ]

fetchPipeline : Concourse.PipelineIdentifier -> Cmd Msg
fetchPipeline pipelineIdentifier =
  Cmd.map PipelineFetched <|
    Task.perform Err Ok (Concourse.Pipeline.fetchPipeline pipelineIdentifier)

fetchUser : Cmd Msg
fetchUser =
  Cmd.map UserFetched <|
    Task.perform Err Ok Concourse.User.fetchUser

logOut : Cmd Msg
logOut =
  Cmd.map LoggedOut <|
    Task.perform Err Ok Concourse.User.logOut
