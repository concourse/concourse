port module TopBar exposing (Model, Msg(..), GroupsState(..), init, update, urlUpdate, view, subscriptions, fetchUser)

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

-- port toggleSidebar : () -> Cmd msg
-- port groupsChanged : List String -> Cmd msg
-- port setViewingPipeline : (Bool -> msg) -> Sub msg

type alias Model =
  { pipelineIdentifier : Maybe Concourse.PipelineIdentifier
  , location : Routes.ConcourseRoute
  , groupsState : GroupsState
  , selectedGroups : List String
  , pipeline : Maybe Concourse.Pipeline
  , userState : UserState
  , userMenuVisible : Bool
  }

type UserState
  = UserStateLoggedIn Concourse.User
  | UserStateLoggedOut
  | UserStateUnknown

type GroupsState
  = GroupsStateSelected (List String)
  | GroupsStateDefault (List String)
  | GroupsStateNotSelected
--
-- type alias Ports =
--   { toggleSidebar : () -> Cmd Msg
--   , setGroups : List String -> Cmd Msg
--   , selectGroups : (List String -> Msg) -> Sub Msg
--   , navigateTo : String -> Cmd Msg
--   , setViewingPipeline : (Bool -> Msg) -> Sub Msg
--   }

type Msg
  = Noop
  | PipelineFetched (Result Http.Error Concourse.Pipeline)
  | UserFetched (Result Concourse.User.Error Concourse.User)
  | FetchPipeline Concourse.PipelineIdentifier
  | ToggleSidebar
  | ToggleGroup Concourse.PipelineGroup
  | SetGroups (List String)
  | SelectGroups (List String)
  | LogOut
  | NavTo String
  | LoggedOut (Result Concourse.User.Error ())
  | ToggleUserMenu

init : Routes.ConcourseRoute -> (Model, Cmd Msg)
init initialLocation =
  let
    queryGroups =
      QueryString.all "groups" initialLocation.queries
    (model, cmd) =
      updateModel
        initialLocation
        { pipelineIdentifier = Nothing
        , groupsState =
            case queryGroups of
              [] ->
                GroupsStateNotSelected
              _ ->
                GroupsStateSelected queryGroups
        , location = initialLocation
        , selectedGroups = queryGroups
        , pipeline = Nothing
        , userState = UserStateUnknown
        , userMenuVisible = False
        }
  in
    flip always (Debug.log ("TopBar.init") ()) <|
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
      let
        firstGroup =
          List.head pipeline.groups

        groups =
          if List.isEmpty model.selectedGroups then
            case firstGroup of
              Nothing ->
                []
              Just group ->
                [group.name]
          else
            model.selectedGroups

        model =
          { model | pipeline = Just pipeline }
      in
        case firstGroup of
          Nothing ->
            (model, Cmd.none)

          Just group ->
            case model.groupsState of
              GroupsStateNotSelected ->
                ( { model |
                    groupsState = GroupsStateDefault [group.name]
                    , pipelineIdentifier =
                      Just {pipelineName = pipeline.name, teamName = pipeline.teamName}}
                , Cmd.none
                )
              GroupsStateDefault groups ->
                ( { model |
                    groupsState = GroupsStateDefault [group.name]
                    , pipelineIdentifier =
                      Just {pipelineName = pipeline.name, teamName = pipeline.teamName}}
                , Cmd.none
                )
              GroupsStateSelected groups ->
                setGroups groups model

    PipelineFetched (Err err) ->
      Debug.log
        ("failed to load pipeline: " ++ toString err)
        (model, Cmd.none)

    ToggleSidebar ->
      Debug.log "sidebar-toggle-incorrectly-handled: " (model, Cmd.none)

    ToggleGroup group ->
      let
        newGroups =
          toggleGroup group [] model.pipeline --TODO add groupState stuff
      in
        setGroups newGroups model

    SetGroups groups ->
      setGroups groups model

    SelectGroups groups ->
      -- setSelectedGroups groups model
      (model, Cmd.none)

    LogOut ->
      (model, logOut)

    LoggedOut (Ok _) ->
      ({ model | userState = UserStateLoggedOut }, Navigation.newUrl "/")

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
  -- flip always (Debug.log "foo" ()) <|
  -- Debug.log ("setGroups: " ++ toString newGroups ++ " - " ++ toString model.selectedGroups) <|
  let
    newUrl =
      pidToUrl model.pipelineIdentifier <|
        setGroupsInLocation model.location newGroups
  in
    (model, Navigation.newUrl newUrl)

urlUpdate : Routes.ConcourseRoute -> Model -> (Model, Cmd Msg)
urlUpdate route model =
  let
    groupsState =
      case route.logical of
        Routes.Home ->
          GroupsStateDefault <| QueryString.all "groups" route.queries
        _ ->
          GroupsStateSelected <| QueryString.all "groups" route.queries
  in
    ( { model
      | groupsState = groupsState
      , location = route
      }
    , Cmd.none
    )

setGroupsInLocation : Routes.ConcourseRoute -> List String -> Routes.ConcourseRoute
setGroupsInLocation loc groups =
  let
    updatedUrl =
      if groups == [] then
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
  flip always (Debug.log ("pid") (pid)) <|
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
toggleGroup group names mpipeline =
  let
    toggled =
      if List.member group.name names then
        List.filter ((/=) group.name) names
      else
        group.name :: names
  in
    defaultToFirstGroup toggled mpipeline

defaultToFirstGroup : List String -> Maybe Concourse.Pipeline -> List String
defaultToFirstGroup groups mpipeline =
  if List.isEmpty groups then
    case mpipeline of
      Just {groups} ->
        List.take 1 (List.map .name groups)

      Nothing ->
        []
  else
    groups

view : Model -> Html Msg
view model =
  -- flip always (Debug.log ("view: " ++ toString model.selectedGroups) ()) <|
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
              case model.groupsState of
                GroupsStateNotSelected ->
                  []
                GroupsStateSelected groups ->
                  List.map
                    (viewGroup groups pipeline.url)
                    pipeline.groups
                GroupsStateDefault groups ->
                  List.map
                    (viewGroup groups pipeline.url)
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
            [ StrictEvents.onLeftClick <| NavTo "/login"
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

-- setGroupsInPipeline : List String -> Cmd Msg
-- setGroupsInPipeline groups =
--   let tsk =
--     Task.perform (always SelectGroups) (always SelectGroups) <|
--       Task.succeed <|
--         SelectGroups groups
--   in
--     tsk
    -- case tsk of
    --   Ok t ->
    --     t
    --   Err err ->
    --     Cmd.none
