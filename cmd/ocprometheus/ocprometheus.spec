Name:           ocprometheus
Version:        0.0.2
Release:        1%{?dist}
Summary:        ocprometheus
License:        Apache2.0
Source0:        https://github.com/aristanetworks/goarista/archive/ocprometheus-v%{version}.tar.gz 
BuildRequires:  go1.14

BuildRoot: %(mktemp -ud %{_tmppath}/%{name}-%{version}-%{release}-XXXXXX)

%description

%prep
%setup -q -n goarista-ocprometheus-v%{version}

%build
    cd cmd/ocprometheus && ( GOOS=linux GOARCH=386 go build )

%install
    mkdir -p %{buildroot}/usr/bin
    cp cmd/ocprometheus/ocprometheus %{buildroot}/usr/bin/

%files
    /usr/bin/ocprometheus

%post
%postun

