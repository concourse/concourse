port module Pipeline exposing (Flags, init, update, view, subscriptions)

import Html exposing (Html)
import Html.Attributes exposing (class, href, id, style)
import Html.Attributes.Aria exposing (ariaLabel)
import Http
import Json.Encode
import Process
import Task
import Time exposing (Time)

import Concourse
import Concourse.Cli
import Concourse.Info
import Concourse.Job
import Concourse.Resource

type alias Flags =
  { teamName : String
  , pipelineName : String
  }

type alias Ports =
  { render : (Json.Encode.Value, Json.Encode.Value) -> Cmd Msg
  , renderFinished : (Bool -> Msg) -> Sub Msg
  }

type alias Model =
  { ports : Ports
  , pipelineLocator : Concourse.PipelineIdentifier
  , fetchedJobs : Maybe Json.Encode.Value
  , fetchedResources : Maybe Json.Encode.Value
  , renderedJobs : Maybe Json.Encode.Value
  , renderedResources : Maybe Json.Encode.Value
  , concourseVersion : String
  }

init : Ports -> Flags -> (Model, Cmd Msg)
init ports flags =
  let
    model =
      { ports = ports
      , pipelineLocator =
          { teamName = flags.teamName
          , pipelineName = flags.pipelineName
          }
      , fetchedJobs = Nothing
      , fetchedResources = Nothing
      , renderedJobs = Nothing
      , renderedResources = Nothing
      , concourseVersion = ""
      }
  in
    ( model
    , Cmd.batch
        [ fetchJobsAfterDelay 0 model.pipelineLocator
        , fetchResourcesAfterDelay 0 model.pipelineLocator
        , fetchVersion
        ]
    )

type Msg
  = Noop
  | AutoupdateVersionTicked Time
  | RenderFinished Bool
  | JobsFetched (Result Http.Error Json.Encode.Value)
  | ResourcesFetched (Result Http.Error Json.Encode.Value)
  | VersionFetched (Result Http.Error String)

update : Msg -> Model -> (Model, Cmd Msg)
update msg model =
  case msg of
    Noop ->
      (model, Cmd.none)

    AutoupdateVersionTicked _ ->
      (model, fetchVersion)

    RenderFinished _ ->
      scheduleResourcesAndJobsFetching model

    JobsFetched (Ok fetchedJobs) ->
      let
        newModel = { model | fetchedJobs = Just fetchedJobs }
      in
        case model.fetchedResources of
          Just fetchedResources ->
            if model.renderedJobs == Just fetchedJobs then
              scheduleResourcesAndJobsFetching newModel
            else
              ( { newModel | renderedJobs = Just fetchedJobs, renderedResources = Just fetchedResources }
              , model.ports.render (fetchedJobs, fetchedResources)
              )
          Nothing ->
            (newModel, Cmd.none)

    JobsFetched (Err err) ->
      Debug.log ("failed to fetch jobs: " ++ toString err) <|
        (model, Cmd.none)

    ResourcesFetched (Ok fetchedResources) ->
      let
        newModel = { model | fetchedResources = Just fetchedResources }
      in
        case model.fetchedJobs of
          Just fetchedJobs ->
              if model.renderedResources == Just fetchedResources then
                scheduleResourcesAndJobsFetching newModel
              else
                ( { newModel | renderedResources = Just fetchedResources, renderedJobs = Just fetchedJobs }
                , model.ports.render (fetchedJobs, fetchedResources)
                )
          Nothing ->
            (newModel, Cmd.none)

    ResourcesFetched (Err err) ->
      Debug.log ("failed to fetch resources: " ++ toString err) <|
        (model, Cmd.none)

    VersionFetched (Ok version) ->
      ({ model | concourseVersion = version }, Cmd.none)

    VersionFetched (Err err) ->
      Debug.log ("failed to fetch version: " ++ toString err) <|
        (model, Cmd.none)

subscriptions : Model -> Sub Msg
subscriptions model =
  Sub.batch
    [ autoupdateVersionTimer
    , model.ports.renderFinished RenderFinished
    ]

view : Model -> Html Msg
view model =
  Html.div []
    [ Html.dl [class "legend"]
        [ Html.dt [class "pending"] []
        , Html.dd [] [Html.text "pending"]
        , Html.dt [class "started"] []
        , Html.dd [] [Html.text "started"]
        , Html.dt [class "succeeded"] []
        , Html.dd [] [Html.text "succeeded"]
        , Html.dt [class "failed"] []
        , Html.dd [] [Html.text "failed"]
        , Html.dt [class "errored"] []
        , Html.dd [] [Html.text "errored"]
        , Html.dt [class "aborted"] []
        , Html.dd [] [Html.text "aborted"]
        , Html.dt [class "paused"] []
        , Html.dd [] [Html.text "paused"]
        ]
    , Html.table [class "lower-right-info"]
        [ Html.tr []
            [ Html.td [class "label"] [ Html.text "cli:"]
            , Html.td []
                [ Html.ul [class "cli-downloads"]
                    [ Html.li []
                        [ Html.a
                            [href (Concourse.Cli.downloadUrl "amd64" "darwin"), ariaLabel "Download OS X CLI"]
                            [ Html.i [class "fa fa-apple"] [] ]
                        ]
                    , Html.li []
                        [ Html.a
                            [href (Concourse.Cli.downloadUrl "amd64" "windows"), ariaLabel "Download Windows CLI"]
                            [ Html.i [class "fa fa-windows"] [] ]
                        ]
                    , Html.li []
                        [ Html.a
                            [href (Concourse.Cli.downloadUrl "amd64" "linux"), ariaLabel "Download Linux CLI"]
                            [ Html.i [class "fa fa-linux"] [] ]
                        ]
                    ]
                ]
            ]
        , Html.tr []
            [ Html.td [class "label"] [ Html.text "version:" ]
            , Html.td []
                [ Html.div [id "concourse-version"]
                    [ Html.text "v"
                    , Html.span [class "number"] [Html.text model.concourseVersion]
                    ]
                ]
            ]
        ]
    ]

autoupdateVersionTimer : Sub Msg
autoupdateVersionTimer =
  Time.every (1 * Time.minute) AutoupdateVersionTicked

scheduleResourcesAndJobsFetching : Model -> (Model, Cmd Msg)
scheduleResourcesAndJobsFetching model =
  ({ model | fetchedResources = Nothing, fetchedJobs = Nothing }
  , Cmd.batch
      [ fetchResourcesAfterDelay (4 * Time.second) model.pipelineLocator
      , fetchJobsAfterDelay (4 * Time.second) model.pipelineLocator
      ]
  )

fetchResourcesAfterDelay : Time -> Concourse.PipelineIdentifier -> Cmd Msg
fetchResourcesAfterDelay delay pid =
  Cmd.map ResourcesFetched << Task.perform Err Ok <|
    Process.sleep delay `Task.andThen` (always <| Concourse.Resource.fetchResourcesRaw pid)

fetchJobsAfterDelay : Time -> Concourse.PipelineIdentifier -> Cmd Msg
fetchJobsAfterDelay delay pid =
  Cmd.map JobsFetched << Task.perform Err Ok <|
    Process.sleep delay `Task.andThen` (always <| Concourse.Job.fetchJobsRaw pid)

fetchVersion : Cmd Msg
fetchVersion =
  Concourse.Info.fetchVersion
    |> Task.perform Err Ok
    |> Cmd.map VersionFetched
