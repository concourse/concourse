module TopBar exposing (..)

import Html exposing (Html)
import Html.Attributes exposing (class, classList, href, id, disabled, attribute, style)
import Html.Events
import Http
import Json.Decode exposing ((:=))
import List
import Navigation exposing (Location)
import String
import Task
import Time

import Concourse.Pipeline exposing (Pipeline, PipelineIdentifier, Group)

type alias Flags =
  { pipeline : Maybe PipelineIdentifier
  , selectedGroups : List String
  , viewingPipeline : Bool
  }

type alias Model =
  { pipelineIdentifier : Maybe PipelineIdentifier
  , pipeline : Maybe Pipeline
  , viewingPipeline : Bool
  , ports : Ports
  , location : Location
  , selectedGroups : List String
  }

type alias Ports =
  { toggleSidebar : () -> Cmd Msg
  , setGroups : List String -> Cmd Msg
  , navigateTo : String -> Cmd Msg
  }

type Msg
  = PipelineFetched (Result Http.Error Pipeline)
  | FetchPipeline PipelineIdentifier
  | ToggleSidebar
  | ToggleGroup Group
  | SetGroup Group

init : Ports -> Flags -> Location -> (Model, Cmd Msg)
init ports flags initialLocation =
  ( { pipelineIdentifier = flags.pipeline
    , viewingPipeline = flags.viewingPipeline
    , ports = ports
    , selectedGroups = flags.selectedGroups
    , location = initialLocation
    , pipeline = Nothing
    }
  , case flags.pipeline of
      Just pid ->
        fetchPipeline pid
      Nothing ->
        Cmd.none
  )

update : Msg -> Model -> (Model, Cmd Msg)
update msg model =
  case msg of
    FetchPipeline pid ->
      (model, fetchPipeline pid)

    PipelineFetched (Ok pipeline) ->
      ( { model | pipeline = Just pipeline }
      , Cmd.none
      )

    PipelineFetched (Err err) ->
      Debug.log
        ("failed to load pipeline: " ++ toString err)
        (model, Cmd.none)

    ToggleSidebar ->
      (model, model.ports.toggleSidebar ())

    ToggleGroup group ->
      let
        newGroups =
          toggleGroup group model.selectedGroups model.pipeline
      in
        setGroups newGroups model

    SetGroup group ->
      setGroups [group.name] model

subscriptions : Model -> Sub Msg
subscriptions model =
  case model.pipelineIdentifier of
    Nothing ->
      Sub.none

    Just pid ->
      Time.every (5 * Time.second) (always (FetchPipeline pid))

setGroups : List String -> Model -> (Model, Cmd Msg)
setGroups newGroups model =
  case model.pipeline of
    Just pipeline ->
      let
        newUrl =
          locationToHistory pipeline <|
            setGroupsInLocation model.location newGroups
      in
        ( { model | selectedGroups = newGroups }
        , if model.viewingPipeline then
            Cmd.batch
              [ Navigation.modifyUrl newUrl
              , model.ports.setGroups newGroups
              ]
          else
            model.ports.navigateTo newUrl
        )

    Nothing ->
      (model, Cmd.none)

urlUpdate : Location -> Model -> (Model, Cmd Msg)
urlUpdate location model =
  ( { model
    | selectedGroups = groupsFromLocation location model.pipeline
    , location = location
    }
  , Cmd.none
  )

groupsFromLocation : Location -> Maybe Pipeline -> List String
groupsFromLocation {search} mpipeline =
  let
    noQuestion =
      String.dropLeft 1 search

    extractGroup query =
      case query of
        ["groups", group] ->
          Just group

        _ ->
          Nothing

    extracted =
      List.filterMap extractGroup <|
        List.map (String.split "=") <|
        String.split "&" noQuestion
    in
      extracted

setGroupsInLocation : Location -> List String -> Location
setGroupsInLocation loc groups =
  let
    noQuestion =
      String.dropLeft 1 loc.search

    nonGroups =
      List.filter (not << String.startsWith "groups=") <|
        List.filter (not << String.isEmpty) <|
        String.split "&" noQuestion

    groupQueries =
      List.map ((++) "groups=") groups

    search =
      String.join "&" (nonGroups ++ groupQueries)
  in
    { loc | search = "?" ++ search }

locationToHistory : Pipeline -> Location -> String
locationToHistory {url} {search,hash} =
  String.join "" [url, search, hash]

toggleGroup : Group -> List String -> Maybe Pipeline -> List String
toggleGroup group names mpipeline =
  let
    toggled =
      if List.member group.name names then
        List.filter ((/=) group.name) names
      else
        group.name :: names
  in
    defaultToFirstGroup toggled mpipeline

defaultToFirstGroup : List String -> Maybe Pipeline -> List String
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
                (viewGroup model.selectedGroups pipeline.url)
                  pipeline.groups
      in
        Html.ul [class "groups"] <|
          [ Html.li [class "main"]
              [ Html.span
                  [ class "sidebar-toggle test btn-hamburger"
                  , Html.Events.onClick ToggleSidebar
                  , Html.Attributes.attribute "aria-label" "Toggle List of Pipelines"
                  ]
                  [ Html.i [class "fa fa-bars"] []
                  ]
              ]
           , Html.li [class "main"]
              [ Html.a
                  [ Html.Attributes.href <|
                      Maybe.withDefault "/" (Maybe.map .url model.pipeline)
                  ]
                  [ Html.i [class "fa fa-home"] []
                  ]
              ]
          ] ++ groupList
    , Html.ul [class "nav-right"]
        [ Html.li [class "nav-item"]
            [ Html.a
                [ href "/builds"
                , Html.Attributes.attribute "aria-label" "Builds List"
                ]
                [ Html.i [class "fa fa-tasks"] []
                ]
            ]
        ]
    ]

isPaused : Maybe Pipeline -> Bool
isPaused =
  Maybe.withDefault False << Maybe.map .paused

viewGroup : List String -> String -> Group -> Html Msg
viewGroup selectedGroups url grp =
  Html.li
    [ if List.member grp.name selectedGroups
        then class "main active"
        else class "main"
    ]
    [ Html.a
        [ Html.Attributes.href <| url ++ "?groups=" ++ grp.name
        , onClickOrShiftClick (SetGroup grp) (ToggleGroup grp)
        ]
        [ Html.text grp.name]
    ]

fetchPipeline : PipelineIdentifier -> Cmd Msg
fetchPipeline pipelineIdentifier =
  Cmd.map PipelineFetched <|
    Task.perform Err Ok (Concourse.Pipeline.fetchPipeline pipelineIdentifier)

onClickOrShiftClick : Msg -> Msg -> Html.Attribute Msg
onClickOrShiftClick clickMsg shiftClickMsg =
  Html.Events.onWithOptions "click"
    { stopPropagation = False, preventDefault = True } <|
      Json.Decode.customDecoder
      (Json.Decode.object2 (,)
        ("button" := Json.Decode.int)
        ("shiftKey" := Json.Decode.bool)) <|
          determineClickMsg clickMsg shiftClickMsg

determineClickMsg : Msg -> Msg -> (Int, Bool) -> Result String Msg
determineClickMsg clickMsg shiftClickMsg (button, shiftKey) =
  case (button, shiftKey) of
    (0, True) ->
      Ok shiftClickMsg
    (0, False) ->
      Ok clickMsg
    _ ->
      Err "placeholder error, nothing is wrong"
