port module Pipeline exposing (Model, Msg, Flags, init, update, view, subscriptions, loadPipeline)

import Html exposing (Html)
import Html.Attributes exposing (class, href, id, style, src, width, height)
import Html.Attributes.Aria exposing (ariaLabel)
import Http
import Json.Encode
import Svg exposing (..)
import Svg.Attributes as SvgAttributes
import Task
import Time exposing (Time)

import Concourse
import Concourse.Cli
import Concourse.Info
import Concourse.Job
import Concourse.Resource

type alias Ports =
  { render : (Json.Encode.Value, Json.Encode.Value) -> Cmd Msg
  }

type alias Model =
  { ports : Ports
  , pipelineLocator : Concourse.PipelineIdentifier
  , fetchedJobs : Maybe Json.Encode.Value
  , fetchedResources : Maybe Json.Encode.Value
  , renderedJobs : Maybe Json.Encode.Value
  , renderedResources : Maybe Json.Encode.Value
  , concourseVersion : String
  , turbulenceImgSrc : String
  , experiencingTurbulence : Bool
  }

type alias Flags =
  { teamName : String
  , pipelineName : String
  , turbulenceImgSrc : String
  }

type Msg
  = Noop
  | AutoupdateVersionTicked Time
  | AutoupdateTimerTicked Time
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
    model =
      { ports = ports
      , concourseVersion = ""
      , turbulenceImgSrc = flags.turbulenceImgSrc
      , pipelineLocator = pipelineLocator
      , fetchedJobs = Nothing
      , fetchedResources = Nothing
      , renderedJobs = Nothing
      , renderedResources = Nothing
      , experiencingTurbulence = False
      }
  in
    loadPipeline pipelineLocator model

loadPipeline : Concourse.PipelineIdentifier -> Model -> (Model, Cmd Msg)
loadPipeline pipelineLocator model =
      ( { model
        | pipelineLocator = pipelineLocator
        , fetchedJobs = Nothing
        , fetchedResources = Nothing
        , renderedJobs = Nothing
        , renderedResources = Nothing
        , experiencingTurbulence = False
        }
      , Cmd.batch
          [ fetchJobs pipelineLocator
          , fetchResources pipelineLocator
          , fetchVersion
          ]
      )

update : Msg -> Model -> (Model, Cmd Msg)
update msg model =
  case msg of
    Noop ->
      (model, Cmd.none)

    AutoupdateTimerTicked timestamp ->
      ( model
      , Cmd.batch
          [ fetchJobs model.pipelineLocator
          , fetchResources model.pipelineLocator
          ]
      )

    AutoupdateVersionTicked _ ->
      (model, fetchVersion)

    JobsFetched (Ok fetchedJobs) ->
      renderIfNeeded { model | fetchedJobs = Just fetchedJobs, experiencingTurbulence = False }

    JobsFetched (Err err) ->
      renderIfNeeded { model | fetchedJobs = Nothing, experiencingTurbulence = True }

    ResourcesFetched (Ok fetchedResources) ->
      renderIfNeeded { model | fetchedResources = Just fetchedResources, experiencingTurbulence = False }

    ResourcesFetched (Err err) ->
      renderIfNeeded { model | fetchedResources = Nothing, experiencingTurbulence = True }

    VersionFetched (Ok version) ->
      ({ model | concourseVersion = version, experiencingTurbulence = False }, Cmd.none)

    VersionFetched (Err err) ->
      Debug.log ("failed to fetch version: " ++ toString err) <|
        ({ model | experiencingTurbulence = True }, Cmd.none)

subscriptions : Model -> Sub Msg
subscriptions model =
  Sub.batch
    [ autoupdateVersionTimer
    , Time.every (5 * Time.second) AutoupdateTimerTicked
    ]

view : Model -> Html Msg
view model =
  Html.div []
    [ Svg.svg
      [ SvgAttributes.class "pipeline-graph test"
      , SvgAttributes.width "100%"
      , SvgAttributes.height "100%"
      ] []
    , Html.div [if model.experiencingTurbulence then class "error-message" else class "error-message hidden"]
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

renderIfNeeded : Model -> (Model, Cmd Msg)
renderIfNeeded model =
  case (model.fetchedResources, model.fetchedJobs) of
    (Just fetchedResources, Just fetchedJobs) ->
      if model.renderedJobs /= Just fetchedJobs || model.renderedResources /= Just fetchedResources then
        ( { model | renderedJobs = Just fetchedJobs, renderedResources = Just fetchedResources }
        , model.ports.render (fetchedJobs, fetchedResources)
        )
      else
        (model, Cmd.none)
    _ ->
      (model, Cmd.none)

fetchResources : Concourse.PipelineIdentifier -> Cmd Msg
fetchResources pid =
  Cmd.map ResourcesFetched << Task.perform Err Ok <| Concourse.Resource.fetchResourcesRaw pid

fetchJobs : Concourse.PipelineIdentifier -> Cmd Msg
fetchJobs pid =
  Cmd.map JobsFetched << Task.perform Err Ok <| Concourse.Job.fetchJobsRaw pid

fetchVersion : Cmd Msg
fetchVersion =
  Concourse.Info.fetchVersion
    |> Task.perform Err Ok
    |> Cmd.map VersionFetched
