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
      -- (model, Cmd.none)
      let
        groupsInUrl =
          QueryString.all "groups" model.location.queries
        groupsState =
          case model.location.logical of
            Routes.Home ->
              getDefaultGroupsState model
            Routes.SelectTeam ->
              GroupsStateNotSelected
            Routes.TeamLogin teamName ->
              GroupsStateNotSelected
            _ ->
              if List.isEmpty groupsInUrl then
                getDefaultGroupsState model
              else
                GroupsStateSelected groupsInUrl
      in
        ( {model | groupsState = groupsState}, fetchPipeline pid)

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
                flip always (Debug.log ("topbar-pipelinefetched") ()) <|
                [group.name]
          else
            model.selectedGroups

        model =
          { model | pipeline = Just pipeline }
      in
        flip always (Debug.log ("PipelineFetched topbar") (pipeline.name)) <|

        case firstGroup of
          Nothing ->
            (model, Cmd.none)

          Just group ->
            case model.groupsState of
              GroupsStateNotSelected ->
                flip always (Debug.log ("GroupsStateNotSelected pipelineName") (pipeline.name)) <|
                (  { model |
                    groupsState = GroupsStateDefault [group.name]
                    , pipelineIdentifier =
                      Just {pipelineName = pipeline.name, teamName = pipeline.teamName}}
                , Cmd.none
                )
              GroupsStateDefault groups ->
                flip always (Debug.log ("GroupsStateDefault pipelineName") (pipeline.name)) <|
                (   { model |
                    groupsState = GroupsStateDefault [group.name]
                    , pipelineIdentifier =
                      Just {pipelineName = pipeline.name, teamName = pipeline.teamName}}
                , Cmd.none
                )
              GroupsStateSelected groups ->
                flip always (Debug.log ("GroupsStateSelcted pipelinename") (pipeline.name)) <|
                setGroups groups model

    PipelineFetched (Err err) ->
      Debug.log
        ("failed to load pipeline: " ++ toString err)
        (model, Cmd.none)

    ToggleSidebar ->
      Debug.log "sidebar-toggle-incorrectly-handled: " (model, Cmd.none)

    ToggleGroup group ->
      flip always (Debug.log ("toggleGroup") ()) <|
      let
        newGroups =
          case model.groupsState of
            GroupsStateNotSelected ->
              flip always (Debug.log ("GroupsStateNotSelected") ()) <|
              toggleGroup group [] model.pipeline -- TODO add groupState stuff
            GroupsStateDefault groups ->
              flip always (Debug.log ("GroupsStateDefault") ()) <|
              toggleGroup group groups model.pipeline
            GroupsStateSelected groups ->
              flip always (Debug.log ("GroupsStateSelected") ()) <|
              toggleGroup group groups model.pipeline
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
      flip always (Debug.log ("tick tobbar nopid") ()) <|
      Sub.none
    Just pid ->
      flip always (Debug.log ("tick tobbar") (pid)) <|
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
  flip always (Debug.log "foo" ()) <|
  -- Debug.log ("setGroups: " ++ toString newGroups ++ " - " ++ toString model.selectedGroups) <|
  let
    newUrl =
      pidToUrl model.pipelineIdentifier <|
        setGroupsInLocation model.location newGroups
  in
    flip always (Debug.log ("setting new url to") (newUrl)) <|
    (model, Navigation.newUrl newUrl)

urlUpdate : Routes.ConcourseRoute -> Model -> (Model, Cmd Msg)
urlUpdate route model =
  flip always (Debug.log "urlUpdate bar" ()) <|
  let
    pipelineIdentifier =
      extractPidFromRoute route.logical
    groupsState =
      GroupsStateNotSelected
  in
    ( { model |
       pipelineIdentifier = pipelineIdentifier
      , location = route
      , groupsState = groupsState
    }
      -- , Cmd.none
    , case pipelineIdentifier of
      Nothing ->
        Cmd.none
      Just pid ->
        fetchPipeline pid
    )



getDefaultGroupsState : Model -> GroupsState
getDefaultGroupsState model =
  case model.pipeline of
    Nothing ->
      GroupsStateNotSelected
    Just pipeline ->
      case List.head pipeline.groups of
        Nothing ->
          GroupsStateNotSelected
        Just first ->
          GroupsStateDefault [first.name]


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
toggleGroup grp names mpipeline =
  if List.member grp.name names then
    List.filter ((/=) grp.name) names
  else
    grp.name :: names


-- defaultToFirstGroup : List String -> Maybe Concourse.Pipeline -> List String
-- defaultToFirstGroup groups mpipeline =
--   if List.isEmpty groups then
--     case mpipeline of
--       Just {groups} ->
--         List.take 1 (List.map .name groups)
--
--       Nothing ->
--         []
--   else
--     groups

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
  flip always (Debug.log ("viewGroup") (selectedGroups)) <|
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
  flip always (Debug.log ("pipelineIdentifier TopBar") (pipelineIdentifier)) <|
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
