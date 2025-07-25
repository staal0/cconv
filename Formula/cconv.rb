# typed: false
# frozen_string_literal: true

# This file was generated by GoReleaser. DO NOT EDIT.
class Cconv < Formula
  desc ""
  homepage ""
  version "1.0.2"

  depends_on "git"

  on_macos do
    if Hardware::CPU.intel?
      url "https://github.com/staal0/cconv/releases/download/v1.0.2/cconv_Darwin_x86_64.tar.gz"
      sha256 "80b0309eb517ea6301ca87c80a55d88d6a1eae22608162558985d30e8ea21498"

      def install
        bin.install "cconv"
      end
    end
    if Hardware::CPU.arm?
      url "https://github.com/staal0/cconv/releases/download/v1.0.2/cconv_Darwin_arm64.tar.gz"
      sha256 "9622a4aaccab719db396f05f1df2ab691e09b1eb07d1fd06ef6f6287543ac3a7"

      def install
        bin.install "cconv"
      end
    end
  end

  on_linux do
    if Hardware::CPU.intel? and Hardware::CPU.is_64_bit?
      url "https://github.com/staal0/cconv/releases/download/v1.0.2/cconv_Linux_x86_64.tar.gz"
      sha256 "cb7cb98a7c7ae832b95b8c68772d6573c817eb26203714466a1ccac1cb9e1146"
      def install
        bin.install "cconv"
      end
    end
    if Hardware::CPU.arm? and Hardware::CPU.is_64_bit?
      url "https://github.com/staal0/cconv/releases/download/v1.0.2/cconv_Linux_arm64.tar.gz"
      sha256 "7f394a5310b832e703d554c452403da495a04606563fe58846d38714c020f085"
      def install
        bin.install "cconv"
      end
    end
  end
end
