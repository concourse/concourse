module Dashboard.Models exposing
    ( DragState(..)
    , DropState(..)
    , Dropdown(..)
    , FetchError(..)
    , FooterModel
    , Model
    )

import Concourse
import Dashboard.Group.Models as GroupModels
import Dict exposing (Dict)
import FetchResult exposing (FetchResult)
import Login.Login as Login
import Message.Effects exposing (Effect(..))
import Message.Message exposing (DropTarget)
import Routes
import Set exposing (Set)
import Time


type alias Model =
    FooterModel
        (Login.Model
            { now : Maybe Time.Posix
            , highDensity : Bool
            , pipelinesWithResourceErrors : Set Concourse.DatabaseID
            , jobs : FetchResult (Dict ( Concourse.DatabaseID, Concourse.JobName ) Concourse.Job)
            , pipelineLayers : Dict Concourse.DatabaseID (List (List Concourse.JobName))
            , teams : FetchResult (List Concourse.Team)
            , dragState : DragState
            , dropState : DropState
            , isJobsRequestFinished : Bool
            , isTeamsRequestFinished : Bool
            , isPipelinesRequestFinished : Bool
            , isResourcesRequestFinished : Bool
            , jobsError : Maybe FetchError
            , teamsError : Maybe FetchError
            , resourcesError : Maybe FetchError
            , pipelinesError : Maybe FetchError
            , viewportWidth : Float
            , viewportHeight : Float
            , scrollTop : Float
            , pipelineJobs : Dict Concourse.DatabaseID (List Concourse.JobName)
            , effectsToRetry : List Effect
            }
        )


type FetchError
    = Failed
    | Disabled


type DragState
    = NotDragging
    | Dragging GroupModels.Card


type DropState
    = NotDropping
    | Dropping DropTarget
    | DroppingWhileApiRequestInFlight Concourse.TeamName


type alias FooterModel r =
    { r
        | hideFooter : Bool
        , hideFooterCounter : Int
        , showHelp : Bool
        , pipelines : Maybe (Dict String (List GroupModels.Pipeline))
        , dropdown : Dropdown
        , highDensity : Bool
        , dashboardView : Routes.DashboardView
        , query : String
    }


type Dropdown
    = Hidden
    | Shown (Maybe Int)
