port module Pipeline exposing (Model, Msg, Flags, init, update, view, subscriptions)

import Html exposing (Html)
import Html.Attributes exposing (class, href, id, style, src, width, height)
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

type alias Ports =
  { render : (Json.Encode.Value, Json.Encode.Value) -> Cmd Msg
  , renderFinished : (Bool -> Msg) -> Sub Msg
  }

type alias Model =
  { ports : Ports
  , pipelineLocator : Concourse.PipelineIdentifier
  , fetchedJobs : FetchedJobsState
  , fetchedResources : FetchedResourcesState
  , renderedJobs : Maybe Json.Encode.Value
  , renderedResources : Maybe Json.Encode.Value
  , concourseVersion : String
  , turbulenceImgSrc : String
  , experiencingTurbulence : Bool
  }

type FetchedResourcesState
  = FetchedResourcesStateScheduled
  | FetchedResourcesStateFetched Json.Encode.Value
  | FetchedResourcesStateFailed

type FetchedJobsState
  = FetchedJobsStateScheduled
  | FetchedJobsStateFetched Json.Encode.Value
  | FetchedJobsStateFailed

type alias Flags =
  { teamName : String
  , pipelineName : String
  , turbulenceImgSrc : String
  }

type Msg
  = Noop
  | AutoupdateVersionTicked Time
  | RenderFinished Bool
  | JobsFetched (Result Http.Error Json.Encode.Value)
  | ResourcesFetched (Result Http.Error Json.Encode.Value)
  | VersionFetched (Result Http.Error String)

init : Ports -> Flags -> (Model, Cmd Msg)
init ports flags =
  let
    pipelineLocator =
      { teamName = flags.teamName
      , pipelineName = flags.pipelineName
      }
  in
    ( { ports = ports
      , pipelineLocator = pipelineLocator
      , fetchedJobs = FetchedJobsStateScheduled
      , fetchedResources = FetchedResourcesStateScheduled
      , renderedJobs = Nothing
      , renderedResources = Nothing
      , concourseVersion = ""
      , turbulenceImgSrc = flags.turbulenceImgSrc
      , experiencingTurbulence = False
      }
    , Cmd.batch
        [ fetchJobsAfterDelay 0 pipelineLocator
        , fetchResourcesAfterDelay 0 pipelineLocator
        , fetchVersion
        ]
    )

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
      renderAndSchedule { model | fetchedJobs = FetchedJobsStateFetched fetchedJobs, experiencingTurbulence = False }

    JobsFetched (Err err) ->
      renderAndSchedule { model | fetchedJobs = FetchedJobsStateFailed, experiencingTurbulence = True }

    ResourcesFetched (Ok fetchedResources) ->
      renderAndSchedule { model | fetchedResources = FetchedResourcesStateFetched fetchedResources, experiencingTurbulence = False }

    ResourcesFetched (Err err) ->
      renderAndSchedule { model | fetchedResources = FetchedResourcesStateFailed, experiencingTurbulence = True }

    VersionFetched (Ok version) ->
      ({ model | concourseVersion = version, experiencingTurbulence = False }, Cmd.none)

    VersionFetched (Err err) ->
      Debug.log ("failed to fetch version: " ++ toString err) <|
        ({ model | experiencingTurbulence = True }, Cmd.none)

subscriptions : Model -> Sub Msg
subscriptions model =
  Sub.batch
    [ autoupdateVersionTimer
    , model.ports.renderFinished RenderFinished
    ]

view : Model -> Html Msg
view model =
  Html.div []
    [ Html.div [if model.experiencingTurbulence then class "error-message" else class "error-message hidden"]
        [ Html.div [class "message"]
            [ Html.img [src model.turbulenceImgSrc, class "seatbelt"] []
            , Html.p [] [Html.text "experiencing turbulence"]
            , Html.p [class "explanation"] []
            ]
        ]
    , Html.dl [class "legend"]
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

renderAndSchedule : Model -> (Model, Cmd Msg)
renderAndSchedule model =
  case model.fetchedResources of
    FetchedResourcesStateFetched fetchedResources ->
      case model.fetchedJobs of
        FetchedJobsStateFetched fetchedJobs ->
          if model.renderedJobs == Just fetchedJobs && model.renderedResources == Just fetchedResources then
            scheduleResourcesAndJobsFetching model
          else
            ( { model | renderedJobs = Just fetchedJobs, renderedResources = Just fetchedResources }
            , model.ports.render (fetchedJobs, fetchedResources)
            )

        FetchedJobsStateScheduled ->
          (model, Cmd.none)

        FetchedJobsStateFailed ->
          scheduleResourcesAndJobsFetching model

    FetchedResourcesStateScheduled ->
      (model, Cmd.none)

    FetchedResourcesStateFailed ->
      scheduleResourcesAndJobsFetching model

scheduleResourcesAndJobsFetching : Model -> (Model, Cmd Msg)
scheduleResourcesAndJobsFetching model =
  ({ model | fetchedResources = FetchedResourcesStateScheduled, fetchedJobs = FetchedJobsStateScheduled }
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
